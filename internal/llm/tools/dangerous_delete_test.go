package tools

import "testing"

// Every command here was auto-approved in an unattended run before 2026-10-05:
// the two text patterns expected one spelling each.
func TestRecursiveDeletesOfSomethingHugeAreRecognisedHoweverTheyAreSpelled(t *testing.T) {
	for _, cmd := range []string{
		`Remove-Item C:\ -Recurse -Force`,
		`Remove-Item -Path C:\ -Recurse`,
		`Remove-Item -Recurse -Force C:\Users\gorilla1`,
		`Remove-Item -Recurse -Force "C:\Users\gorilla1\Documents"`,
		`Remove-Item -Recurse $HOME`,
		`ri -Recurse -Force $env:USERPROFILE\*`,
		`rd /s /q C:\Users\gorilla1`,
		`rd /s /q C:\Users`,
		`del /s /q C:\*`,
		`rmdir /s /q C:\Windows`,
		`rm -rf ~/`,
		`rm -rf $HOME/`,
		`rm -rf ${HOME}/*`,
		`rm -rf /home/someone`,
		`rm -rf /home/someone/.ssh`,
		`rm -rf -- /`,
		`rm -rf /*`,
		`rm -r -f /usr`,
		`rm --recursive --force /etc`,
		`echo ok; sudo rm -rf /var`,
		`cd /tmp && rm -fr '/root'`,
	} {
		if got := DangerousPatternIn(cmd); got == nil {
			t.Errorf("%q is not recognised as irreversible", cmd)
		}
	}
}

// ... and ordinary cleaning is left alone. A check that fires on `rm -rf
// node_modules` is a check people switch off.
func TestOrdinaryRecursiveDeletesAreNotCalledHuge(t *testing.T) {
	for _, cmd := range []string{
		`rm -rf node_modules`,
		`rm -rf ./build dist`,
		`rm -rf *`,
		`rm -rf /tmp/build-1234`,
		`rm -rf /home/someone/project/build`,
		`rm -rf ~/project/.cache`,
		`rm -f /etc`, // not recursive: rm refuses a directory by itself
		`Remove-Item -Recurse -Force .\bin`,
		`Remove-Item -Recurse -Force C:\Users\gorilla1\Documents\proj\out`,
		`rd /s /q build`,
		`del /q C:\temp\x.log`,
		`grep -r "rm -rf /" docs`,
		`echo "Remove-Item -Recurse C:\"`,
	} {
		if deletesSomethingHuge(cmd) {
			t.Errorf("%q is ordinary work and was called a huge delete", cmd)
		}
	}
}
