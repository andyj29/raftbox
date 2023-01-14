package core

type LogEntry struct {
	Index   int
	Term    int
	Command interface{}
}
