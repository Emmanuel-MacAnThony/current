package domain

// Operator is how a subscription reacts to one change: Apply returns the delta it
// computed in memory plus ok — ok=true means "here's the delta"; ok=false means
// "I can't do this incrementally, re-eval me" (the engine re-runs the query and
// diffs). The concrete operators (Filter, ReEval, later max) live in the plan
// package; the interface lives here so a Subscription can hold one without the
// domain importing plan — same reason Connection lives here.
type Operator interface {
	Apply(event ChangeEvent, current ResultSet) (Delta, bool)
}
