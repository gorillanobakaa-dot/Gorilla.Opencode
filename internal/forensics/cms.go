package forensics

// GORILLA (2026-10-22): signer names from a PKCS#7 / CMS SignedData blob, for
// PE Authenticode and the Mach-O code signature. This READS names; it does not
// verify anything (no chain, no hash, no time). Real verification is a
// separate job and must not be implied by this output.

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"math/big"
	"strings"
)

var oidSignedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}

type cmsSigner struct {
	Subject string // empty when the signer's certificate is not in the blob
	Issuer  string
}

type cmsInfo struct {
	Certs   int
	Signers []cmsSigner
}

// derChildren splits the content of a constructed DER value into its elements.
func derChildren(b []byte) ([]asn1.RawValue, error) {
	var out []asn1.RawValue
	for len(b) > 0 {
		var v asn1.RawValue
		rest, err := asn1.Unmarshal(b, &v)
		if err != nil {
			return out, err
		}
		out = append(out, v)
		b = rest
	}
	return out, nil
}

// parseCMSSigners reads the certificates and signer identities of a
// ContentInfo holding SignedData.
func parseCMSSigners(der []byte) (cmsInfo, error) {
	var info cmsInfo
	var ci asn1.RawValue
	if _, err := asn1.Unmarshal(der, &ci); err != nil {
		return info, fmt.Errorf("not DER: %w", err)
	}
	top, err := derChildren(ci.Bytes)
	if err != nil || len(top) < 2 {
		return info, fmt.Errorf("not a PKCS#7 ContentInfo")
	}
	var oid asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(top[0].FullBytes, &oid); err != nil || !oid.Equal(oidSignedData) {
		return info, fmt.Errorf("content type %v is not SignedData", oid)
	}
	if top[1].Class != asn1.ClassContextSpecific || top[1].Tag != 0 {
		return info, fmt.Errorf("SignedData content missing")
	}
	var sd asn1.RawValue
	if _, err := asn1.Unmarshal(top[1].Bytes, &sd); err != nil {
		return info, fmt.Errorf("SignedData unreadable: %w", err)
	}
	fields, err := derChildren(sd.Bytes)
	if err != nil || len(fields) < 4 {
		return info, fmt.Errorf("SignedData too short")
	}
	var certs []*x509.Certificate
	var signerSet asn1.RawValue
	for _, fv := range fields[3:] { // after version, digestAlgorithms, encapContentInfo
		switch {
		case fv.Class == asn1.ClassContextSpecific && fv.Tag == 0: // [0] IMPLICIT certificates
			elems, _ := derChildren(fv.Bytes)
			for _, e := range elems {
				info.Certs++
				if c, err := x509.ParseCertificate(e.FullBytes); err == nil {
					certs = append(certs, c)
				}
			}
		case fv.Class == asn1.ClassUniversal && fv.Tag == asn1.TagSet:
			signerSet = fv
		}
	}
	signers, _ := derChildren(signerSet.Bytes)
	for _, si := range signers {
		sf, err := derChildren(si.Bytes)
		if err != nil || len(sf) < 2 {
			continue
		}
		info.Signers = append(info.Signers, matchSigner(sf[1], certs))
	}
	if len(info.Signers) == 0 {
		return info, fmt.Errorf("no signer in SignedData")
	}
	return info, nil
}

// matchSigner finds the signer's certificate by issuer and serial number, or by
// subject key identifier ([0]), and names it.
func matchSigner(sid asn1.RawValue, certs []*x509.Certificate) cmsSigner {
	if sid.Class == asn1.ClassContextSpecific && sid.Tag == 0 {
		for _, c := range certs {
			if len(c.SubjectKeyId) > 0 && bytes.Equal(c.SubjectKeyId, sid.Bytes) {
				return cmsSigner{Subject: c.Subject.String(), Issuer: c.Issuer.String()}
			}
		}
		return cmsSigner{Issuer: fmt.Sprintf("(by key id %x)", sid.Bytes)}
	}
	parts, err := derChildren(sid.Bytes)
	if err != nil || len(parts) < 2 {
		return cmsSigner{}
	}
	serial := new(big.Int)
	var n *big.Int
	if _, err := asn1.Unmarshal(parts[1].FullBytes, &n); err == nil {
		serial = n
	}
	for _, c := range certs {
		if bytes.Equal(c.RawIssuer, parts[0].FullBytes) && c.SerialNumber.Cmp(serial) == 0 {
			return cmsSigner{Subject: c.Subject.String(), Issuer: c.Issuer.String()}
		}
	}
	var rdn pkix.RDNSequence
	issuer := "(unreadable)"
	if _, err := asn1.Unmarshal(parts[0].FullBytes, &rdn); err == nil {
		var name pkix.Name
		name.FillFromRDNSequence(&rdn)
		issuer = name.String()
	}
	return cmsSigner{Issuer: issuer}
}

// writeCMSSigners prints the signer names indented under a signature line.
func writeCMSSigners(b *strings.Builder, der []byte) {
	info, err := parseCMSSigners(der)
	if err != nil {
		fmt.Fprintf(b, "  signer: unreadable (%s)\n", clean(err.Error()))
		return
	}
	for _, s := range info.Signers {
		if s.Subject == "" {
			fmt.Fprintf(b, "  signer: certificate not in the block; issuer %s\n", clean(s.Issuer))
			continue
		}
		fmt.Fprintf(b, "  signer: %s\n  signer issuer: %s\n", clean(s.Subject), clean(s.Issuer))
	}
	fmt.Fprintf(b, "  certificates in the block: %d\n", info.Certs)
}
