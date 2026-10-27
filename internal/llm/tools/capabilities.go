package tools

// CapabilityStatus is what `gorilla-opencode doctor` prints under "Tool
// capabilities": one line per route a tool can take on this machine, each
// saying what is there or what is missing.
func CapabilityStatus() []string {
	return append(WebSearchStatus(), OCRStatus()...)
}
