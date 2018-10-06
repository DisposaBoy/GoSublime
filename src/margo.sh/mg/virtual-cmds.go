package mg

const (
	// RcActuate is the command that's run when a user triggeres an action
	// at the cursor, primarily via the mouse e.g. goto.definition
	//
	// Args:
	// -button="": the action wasn't triggered by a mouse click
	// -button="left": the action was triggered by the by a left-click
	// -button="right": the action was triggered by the by a right-click
	RcActuate = ".actuate"
)
