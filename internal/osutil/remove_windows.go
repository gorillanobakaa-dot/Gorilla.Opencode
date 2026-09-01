//go:build windows

package osutil

import (
	"os"
	"time"
)

// Remove provides a retry-with-backoff implementation of os.Remove
// to handle transient access denied errors from AV or indexers on Windows.
func Remove(name string) error {
	var err error
	for i := 0; i < 5; i++ {
		err = os.Remove(name)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		time.Sleep(time.Duration(10*i) * time.Millisecond)
	}
	return err
}

// RemoveAll provides a retry-with-backoff implementation of os.RemoveAll
func RemoveAll(path string) error {
	var err error
	for i := 0; i < 5; i++ {
		err = os.RemoveAll(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		time.Sleep(time.Duration(20*i) * time.Millisecond)
	}
	return err
}
