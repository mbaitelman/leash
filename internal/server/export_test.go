package server

// LockRunMu locks the run mutex so tests can simulate a run in progress.
// Returns an unlock func.
func (s *Server) LockRunMu() func() {
	s.runMu.Lock()
	return s.runMu.Unlock
}
