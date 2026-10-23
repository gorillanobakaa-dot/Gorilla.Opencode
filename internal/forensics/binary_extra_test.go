package forensics

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"debug/macho"
	"encoding/asn1"
	"encoding/binary"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exports, version info and signer names (night run 2026-10-22, job 2). The
// fixtures are built from testdata/fxlib.c, .def and .rc; testdata/README.md
// has the commands. No signed fixture can be made offline, so the signer
// tests build a minimal PKCS#7 SignedData here.

func TestPEExportsAndForwarder(t *testing.T) {
	out := mustInspect(t, "testdata/fxlib.pedll", Options{Part: PartExports})
	wantAll(t, out,
		"export name: fxlib.dll",
		"exported functions (ordinal, RVA or -> forwarder, name): 5",
		"ord 9 0x1014 (no name, ordinal only)",
		"ord 10 0x1000 gorilla_export_add",
		"ord 11 0x10000 gorilla_export_data",
		"ord 12 0x1009 gorilla_export_mul",
		"ord 13 -> KERNEL32.GetTickCount gorilla_forwarded",
	)
	f := mustInspect(t, "testdata/fxlib.pedll", Options{Part: PartExports, Filter: "MUL"})
	wantAll(t, f, `matching "MUL": 1 of 5`)
	if strings.Contains(f, "gorilla_export_add") {
		t.Errorf("filter let gorilla_export_add through:\n%s", f)
	}
	// An EXE with no export directory says so instead of failing.
	wantAll(t, mustInspect(t, "testdata/fx.pe", Options{Part: PartExports}), "exported functions (ordinal, RVA or -> forwarder, name): 0")
}

func TestPEVersionInfoInSummary(t *testing.T) {
	out := mustInspect(t, "testdata/fxlib.pedll", Options{})
	wantAll(t, out,
		"format: PE32+ (64-bit) DLL",
		"exports: 5 as fxlib.dll (part=exports lists them)",
		"version info (written by whoever built the file):\n"+
			"  FileVersion: 1.2.3.4\n"+
			"  ProductVersion: 5.6\n"+
			"  CompanyName: Gorilla Fixture Co\n"+
			"  ProductName: Gorilla Fixture\n"+
			"  OriginalFilename: fxlib.dll\n"+
			"  FileDescription: binary_inspect fixture\n"+
			"  numeric: file 1.2.3.4, product 5.6.0.0\n",
		"signature: none",
	)
	if strings.Contains(mustInspect(t, "testdata/fx.pe", Options{}), "version info") {
		t.Error("fx.pe has no version resource but one was reported")
	}
}

// A damaged resource must never panic or loop: every truncation of the real one is parsed.
func TestVersionInfoTruncated(t *testing.T) {
	data := versionResourceBytes(t)
	for n := 0; n < len(data); n++ {
		parseVersionInfo(data[:n])
	}
	if vi := parseVersionInfo(data); vi.Strings["CompanyName"] != "Gorilla Fixture Co" {
		t.Errorf("full resource: %+v", vi)
	}
}

func versionResourceBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/fxlib.pedll")
	if err != nil {
		t.Fatal(err)
	}
	key := utf16le("VS_VERSION_INFO")
	i := strings.Index(string(b), string(key))
	if i < 6 {
		t.Fatal("no VS_VERSION_INFO in the fixture")
	}
	start := i - 6
	return b[start : start+int(binary.LittleEndian.Uint16(b[start:]))]
}

func TestELFExports(t *testing.T) {
	out := mustInspect(t, "testdata/fxlib.elfso", Options{Part: PartExports})
	wantAll(t, out,
		"exported dynamic symbols: 4",
		"func gorilla_export_add",
		"func gorilla_export_mul",
		"data gorilla_export_data",
	)
	if strings.Contains(out, "gorilla_hidden") {
		t.Errorf("a static function was listed as exported:\n%s", out)
	}
	wantAll(t, mustInspect(t, "testdata/fxlib.elfso", Options{}), "exports: 4 (part=exports lists them)")
	// What an executable imports is not what it exports.
	if exe := mustInspect(t, "testdata/fx.elf", Options{Part: PartExports}); strings.Contains(exe, "puts") {
		t.Errorf("an undefined (imported) symbol was listed as exported:\n%s", exe)
	}
}

func TestMachOExportsFromTrie(t *testing.T) {
	out := mustInspect(t, "testdata/fxlib.machodylib", Options{Part: PartExports})
	wantAll(t, out,
		"exported symbols (export trie): 6",
		"0x440 _gorilla_export_add",
		"0x2008 _gorilla_export_data",
		"0x450 _gorilla_export_mul",
	)
	wantAll(t, mustInspect(t, "testdata/fxlib.machodylib", Options{}), "exports: 6 from the export trie")
}

// trieNode builds one export-trie node: terminal bytes, then edges.
func trieNode(term []byte, edges ...any) []byte {
	out := []byte{byte(len(term))}
	out = append(out, term...)
	out = append(out, byte(len(edges)/2))
	for i := 0; i < len(edges); i += 2 {
		out = append(out, edges[i].(string)...)
		out = append(out, 0, byte(edges[i+1].(int)))
	}
	return out
}

func TestExportTrieFlagsAndLoops(t *testing.T) {
	// root(0) -> "_a"(14) plain at 0x10, "_b"(18) weak at 0x20, "_c"(22) re-export of _orig from library 1.
	root := trieNode(nil, "_a", 14, "_b", 18, "_c", 22)
	if len(root) != 14 {
		t.Fatalf("root is %d bytes", len(root))
	}
	trie := append(root, trieNode([]byte{0, 0x10})...)
	trie = append(trie, trieNode([]byte{exportWeak, 0x20})...)
	trie = append(trie, trieNode(append([]byte{exportReexport, 1}, "_orig\x00"...))...)
	got := strings.Join(walkExportTrie(trie), "\n")
	want := "0x10 _a\n0x20 _b (weak)\nre-export of _orig from library 1: _c"
	if got != want {
		t.Errorf("trie walk:\n%s\nwant:\n%s", got, want)
	}

	// A node pointing back at the root must stop at once, not run to the node cap.
	loop := trieNode(nil, "_x", 0)
	lines := walkExportTrie(loop)
	if len(lines) > 3 || !strings.Contains(strings.Join(lines, "\n"), "damaged") {
		t.Errorf("loop trie: %d lines, first %q", len(lines), lines[0])
	}
	for n := 0; n < len(trie); n++ {
		walkExportTrie(trie[:n]) // truncations never panic
	}
}

func TestMachOSymtabExportsFallback(t *testing.T) {
	f := &macho.File{Symtab: &macho.Symtab{Syms: []macho.Symbol{
		{Name: "_exported", Type: 0x0f, Value: 0x100}, // N_SECT|N_EXT
		{Name: "_local", Type: 0x0e, Value: 0x200},    // N_SECT, not external
		{Name: "_imported", Type: 0x01},               // N_UNDF|N_EXT
		{Name: "_debug", Type: 0x24 | 0x01},           // a stab
	}}}
	lines, src := machoExports(f)
	if src != "symbol table, external and defined" || strings.Join(lines, "|") != "0x100 _exported" {
		t.Errorf("fallback: %q from %s", lines, src)
	}
}

// ── signer names ────────────────────────────────────────────────────────────

func tlv(tag byte, parts ...[]byte) []byte {
	var body []byte
	for _, p := range parts {
		body = append(body, p...)
	}
	n := len(body)
	switch {
	case n < 0x80:
		return append([]byte{tag, byte(n)}, body...)
	case n < 0x100:
		return append([]byte{tag, 0x81, byte(n)}, body...)
	default:
		return append([]byte{tag, 0x82, byte(n >> 8), byte(n)}, body...)
	}
}

func mustDER(t *testing.T, v any) []byte {
	t.Helper()
	b, err := asn1.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type testPKI struct {
	root, leaf *x509.Certificate
}

func makePKI(t *testing.T) testPKI {
	t.Helper()
	mk := func(cn string, serial int64, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, ca bool) (*x509.Certificate, *ecdsa.PrivateKey) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		tpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn, Organization: []string{"Gorilla Fixture"}},
			NotBefore: time.Unix(0, 0), NotAfter: time.Unix(0, 0).Add(time.Hour),
			IsCA: ca, BasicConstraintsValid: true, SubjectKeyId: []byte{byte(serial), 0xaa},
		}
		if parent == nil {
			parent, parentKey = tpl, key
		}
		der, err := x509.CreateCertificate(rand.Reader, tpl, parent, &key.PublicKey, parentKey)
		if err != nil {
			t.Fatal(err)
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return c, key
	}
	root, rootKey := mk("Gorilla Test Root", 1, nil, nil, true)
	leaf, _ := mk("Gorilla Test Signer", 2, root, rootKey, false)
	return testPKI{root, leaf}
}

// signedData builds a ContentInfo{SignedData} holding certs (root first, so a
// wrong match would name the root) and one SignerInfo with the given sid.
func signedData(t *testing.T, sid []byte, certs ...*x509.Certificate) []byte {
	t.Helper()
	alg := tlv(0x30, mustDER(t, asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}))
	var raw [][]byte
	for _, c := range certs {
		raw = append(raw, c.Raw)
	}
	signer := tlv(0x30, mustDER(t, 1), sid, alg, tlv(0x30, mustDER(t, asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2})), tlv(0x04, []byte{1, 2, 3}))
	sd := tlv(0x30, mustDER(t, 1), tlv(0x31, alg), tlv(0x30, mustDER(t, asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1})),
		tlv(0xa0, raw...), tlv(0x31, signer))
	return tlv(0x30, mustDER(t, oidSignedData), tlv(0xa0, sd))
}

func byIssuerSerial(t *testing.T, c *x509.Certificate) []byte {
	return tlv(0x30, c.RawIssuer, mustDER(t, c.SerialNumber))
}

func TestCMSSignerByIssuerAndSerial(t *testing.T) {
	p := makePKI(t)
	info, err := parseCMSSigners(signedData(t, byIssuerSerial(t, p.leaf), p.root, p.leaf))
	if err != nil {
		t.Fatal(err)
	}
	if info.Certs != 2 || len(info.Signers) != 1 ||
		info.Signers[0].Subject != "CN=Gorilla Test Signer,O=Gorilla Fixture" ||
		info.Signers[0].Issuer != "CN=Gorilla Test Root,O=Gorilla Fixture" {
		t.Errorf("signers: %+v", info)
	}
}

func TestCMSSignerByKeyIDAndMissingCert(t *testing.T) {
	p := makePKI(t)
	info, err := parseCMSSigners(signedData(t, tlv(0x80, p.leaf.SubjectKeyId), p.root, p.leaf))
	if err != nil || info.Signers[0].Subject != "CN=Gorilla Test Signer,O=Gorilla Fixture" {
		t.Errorf("by key id: %+v %v", info, err)
	}
	// The signer's own certificate left out: its issuer is still named, from the sid.
	var b strings.Builder
	writeCMSSigners(&b, signedData(t, byIssuerSerial(t, p.leaf), p.root))
	wantAll(t, b.String(), "signer: certificate not in the block; issuer CN=Gorilla Test Root,O=Gorilla Fixture", "certificates in the block: 1")
	if _, err := parseCMSSigners([]byte{0x30, 0x03, 0x02, 0x01, 0x01}); err == nil {
		t.Error("a non-PKCS#7 blob was accepted")
	}
	// The right content type with no content after it: refused, not read past its end.
	if _, err := parseCMSSigners(tlv(0x30, mustDER(t, oidSignedData))); err == nil {
		t.Error("a ContentInfo without content was accepted")
	}
}

// fx.pe with an Authenticode block appended and the security directory pointed at it.
func TestPEAuthenticodeSignerNotVerified(t *testing.T) {
	p := makePKI(t)
	img, err := os.ReadFile("testdata/fx.pe")
	if err != nil {
		t.Fatal(err)
	}
	for len(img)%8 != 0 {
		img = append(img, 0)
	}
	blob := signedData(t, byIssuerSerial(t, p.leaf), p.root, p.leaf)
	off := len(img)
	cert := make([]byte, 8, 8+len(blob))
	binary.LittleEndian.PutUint32(cert, uint32(8+len(blob)))
	binary.LittleEndian.PutUint16(cert[4:], 0x0200)
	binary.LittleEndian.PutUint16(cert[6:], winCertTypePKCSSigned)
	img = append(img, append(cert, blob...)...)
	opt := int(binary.LittleEndian.Uint32(img[0x3c:])) + 24
	if binary.LittleEndian.Uint16(img[opt:]) != 0x20b {
		t.Fatal("fx.pe is not PE32+")
	}
	dd := opt + 112 + 8*peDirSecurity
	binary.LittleEndian.PutUint32(img[dd:], uint32(off))
	binary.LittleEndian.PutUint32(img[dd+4:], uint32(8+len(blob)))
	path := filepath.Join(t.TempDir(), "signed.pe")
	if err := os.WriteFile(path, img, 0o644); err != nil {
		t.Fatal(err)
	}
	wantAll(t, mustInspect(t, path, Options{}),
		"signature: Authenticode, NOT verified (chain and hash not checked)\n"+
			"  signer: CN=Gorilla Test Signer,O=Gorilla Fixture\n"+
			"  signer issuer: CN=Gorilla Test Root,O=Gorilla Fixture\n"+
			"  certificates in the block: 2\n")
}

func TestMachOCodeSignature(t *testing.T) {
	// The fixture's arm64 slice is ad-hoc signed by the linker.
	out := mustInspect(t, "testdata/fx-fat.macho", Options{})
	wantAll(t, out,
		"code signature: present, NOT verified (chain and hash not checked)\n"+
			"  identifier: fx-arm64.macho (written by whoever signed it)\n"+
			"  ad-hoc: no signer certificate")

	// A SuperBlob with a CodeDirectory and a CMS blob, as a Developer ID signature has.
	p := makePKI(t)
	cms := signedData(t, byIssuerSerial(t, p.leaf), p.root, p.leaf)
	be := binary.BigEndian
	cd := make([]byte, 44)
	be.PutUint32(cd, csMagicCodeDir)
	be.PutUint32(cd[20:], 44) // identOffset
	cd = append(cd, "org.gorilla.fixture\x00"...)
	be.PutUint32(cd[4:], uint32(len(cd)))
	wrap := make([]byte, 8, 8+len(cms))
	be.PutUint32(wrap, csMagicCMS)
	be.PutUint32(wrap[4:], uint32(8+len(cms)))
	wrap = append(wrap, cms...)
	sb := make([]byte, 28)
	be.PutUint32(sb, csMagicEmbedded)
	be.PutUint32(sb[8:], 2)
	be.PutUint32(sb[12:], csSlotCodeDir)
	be.PutUint32(sb[16:], 28)
	be.PutUint32(sb[20:], csSlotCMS)
	be.PutUint32(sb[24:], uint32(28+len(cd)))
	sb = append(append(sb, cd...), wrap...)
	be.PutUint32(sb[4:], uint32(len(sb)))
	var b strings.Builder
	writeCodeSignature(&b, sb)
	wantAll(t, b.String(),
		"  identifier: org.gorilla.fixture (written by whoever signed it)\n"+
			"  signer: CN=Gorilla Test Signer,O=Gorilla Fixture\n"+
			"  signer issuer: CN=Gorilla Test Root,O=Gorilla Fixture\n")
	if strings.Contains(b.String(), "ad-hoc") {
		t.Errorf("a certificate signature was called ad-hoc:\n%s", b.String())
	}
}
