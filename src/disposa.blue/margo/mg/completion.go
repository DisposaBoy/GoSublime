package mg

type CompletionTag string

const (
	SnippetTag  = CompletionTag("·ʂ")
	VariableTag = CompletionTag("·ν")
	TypeTag     = CompletionTag("·ʈ")
	ConstantTag = CompletionTag("·Ɩ")
	FunctionTag = CompletionTag("·ƒ")
	PackageTag  = CompletionTag("·ρ")
)

type Completion struct {
	Query string
	Title string
	Src   string
	Tag   CompletionTag
}
