package floatwin

// Hooks connects float UI actions to the desktop shell.
type Hooks struct {
	OnCaptured       func(term string)
	OnToggleNotebook func()
}

const (
	floatW = 420
	floatH = 96
)
