package tools

import "testing"

// Owner 2026-11-14: these exact commands raised dialogs on a long kernel
// diagnosis. Reads never ask; the writes next to them still do.
func TestLongTaskReadsNeverAsk(t *testing.T) {
	for _, cmd := range []string{
		"which iw rfkill; iw reg get; rfkill list",
		"diff -rq /a /b | grep -v foo",
		"diff -u /a/x /b/x",
		"grep -E 'CONFIG_CFG80211|CONFIG_MAC80211' /boot/config-*",
		"ls /sys/class/rfkill/ && cat /sys/class/rfkill/rfkill0/state",
		"ip -br link; iw dev; lspci -nnk | grep -i -A3 network",
		"lsusb; lsmod | grep -c iwl",
		"PATH=/usr/sbin:/sbin:$PATH iw reg get",
		"for f in /sys/class/rfkill/*; do cat $f/name; done",
		"dmesg | grep -i firmware | tail -20",
		"journalctl -k -b | grep iwlwifi",
		"lsblk -o NAME,SIZE",
		"grep -rn remove docs",
		"ss -tlnp 2>/dev/null",
		"iw dev wlan0 link",
	} {
		if !IsSafeReadOnly(cmd, readOnlyList()) {
			t.Errorf("%q asks; it only reads", cmd)
		}
	}
}

func TestLongTaskWritesStillAsk(t *testing.T) {
	for _, cmd := range []string{
		"iw dev wlan0 set type monitor",
		"ip link set wlan0 down",
		"ip addr add 10.0.0.1/24 dev eth0",
		"rfkill block wifi",
		"dmesg -C",
		"journalctl --vacuum-size=1M",
		"LD_PRELOAD=/tmp/x.so ls",
		"GIT_EXTERNAL_DIFF=/tmp/x git diff",
		"for f in a; do rm $f; done",
		"for f in a; do cat $f > /etc/x; done",
		"sort -o out in",
		"git branch -d x",
		"ethtool -s eth0 speed 10",
		"cat ~/.ssh/id_rsa",
		"ip netns exec foo sh",
	} {
		if IsSafeReadOnly(cmd, readOnlyList()) {
			t.Errorf("%q would not ask; it writes or is sensitive", cmd)
		}
	}
}
