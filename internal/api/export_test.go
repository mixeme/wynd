package api

// HoldArchiveBuildForTest занимает семафор «одна сборка архива на учётку»,
// как идущая сборка; возвращает освобождение.
func (s *Server) HoldArchiveBuildForTest(accountID string) func() {
	s.archiveBuilds.Store(accountID, struct{}{})
	return func() { s.archiveBuilds.Delete(accountID) }
}
