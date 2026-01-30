package ui

// UserInterface defines how the download engine communicates with the user
type UserInterface interface {
	PrintProgress(gid string, total, completed int64, speed int, numConns int)
	ClearLine()
	Printf(format string, a ...interface{})
	Println(a ...interface{})
}
