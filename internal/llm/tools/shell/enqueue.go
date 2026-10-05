package shell

// enqueue hands a command to the shell's worker. It reports false, instead of
// panicking, when the queue has been closed because the shell process ended.
func (s *PersistentShell) enqueue(c *commandExecution) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	s.commandQueue <- c
	return true
}
