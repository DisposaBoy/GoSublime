package mg

// QueryUserCmds is the action dispatched to get a list of UserCmds.
type QueryUserCmds struct{ ActionType }

// QueryTestCmds is the action dispatched to get a list of UserCmds for testing, benchmarking, etc.
type QueryTestCmds struct{ ActionType }

// UserCmdList is a list of UserCmd
type UserCmdList []UserCmd

// Len implements sort.Interface
func (uc UserCmdList) Len() int {
	return len(uc)
}

// Len implements sort.Interface using UserCmd.Title for comparison
func (uc UserCmdList) Less(i, j int) bool {
	return uc[i].Title < uc[j].Title
}

// Len implements sort.Interface
func (uc UserCmdList) Swap(i, j int) {
	uc[i], uc[j] = uc[j], uc[i]
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

	// Prompts is a list of titles for prompting the user for input before running the command.
	// The user is prompted once for each entry.
	// The inputs are assigned directly to RunCmd.Prompts for command consumption.
	Prompts []string
}
