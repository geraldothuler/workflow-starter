package runner

// AskResolver prompts the user interactively with a Socratic why_ask message.
type AskResolver struct{}

func (r *AskResolver) Name() string { return "ask" }

func (r *AskResolver) Resolve(spec InputSpec, inputs RunInputs, ctx ResolveContext) (string, bool) {
	if ctx.Term == nil || !ctx.Term.CanAsk() {
		return "", false
	}

	val := ctx.Term.Ask(spec.Description, spec.WhyAsk)
	return val, val != ""
}
