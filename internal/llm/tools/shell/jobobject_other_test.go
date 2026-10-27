//go:build !windows

package shell

import "testing"

// The Job Object tests are Windows-only: Unix stops a tree with process
// groups and pkill, which the tests in shell_test.go cover.
const noJobObjects = "Job Objects are a Windows API; Unix uses process groups (see shell_test.go)"

func TestGrandchildSurvivesWithoutJobObject(t *testing.T)                       { t.Skip(noJobObjects) }
func TestJobObjectKillsGrandchild(t *testing.T)                                 { t.Skip(noJobObjects) }
func TestJobObjectGrandchildDiesOnParentCrash(t *testing.T)                     { t.Skip(noJobObjects) }
func TestJobObjectInterruptKillsOrphanedGrandchildAndKeepsSession(t *testing.T) { t.Skip(noJobObjects) }
func TestJobObjectShellCloseKillsBackgroundGrandchild(t *testing.T)             { t.Skip(noJobObjects) }
