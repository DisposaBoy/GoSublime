package mg

// QueryUserCmds is the action dispatched to get a list of UserCmds.
type QueryUserCmds struct {
	ActionType
}

// UserCmd represents a command that may be displayed in the editor ui.
type UserCmd struct {
	// Title is the name of the command displayed to the user
	Title string

	// Desc describes what the command invocation does
	Desc string

	// Name is the name of the command to run
	Name string

	// Args is a list of args to pass to the command
	Args []string
}
