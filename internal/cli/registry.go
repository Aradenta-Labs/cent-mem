package cli

import "sort"

// Command describes a registered subcommand. It carries the metadata needed to
// keep the CLI, docs/cli-contract.md, and skill/SKILL.md in lockstep (the
// "docs are the contract" guarantee enforced by contract tests).
type Command struct {
	Name  string   // subcommand name, e.g. "recall"
	Usage string   // one-line usage, e.g. "recall <query> [flags]"
	Flags []string // declared flag names, e.g. []string{"--scope","--top"}
}

// Registry is an ordered, unique collection of CLI commands. Both main.go and
// the contract tests consume it so there is a single source of truth.
type Registry struct {
	cmds map[string]Command
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{cmds: make(map[string]Command)}
}

// Register adds a command. Registering the same name twice overwrites the prior
// entry; callers that need uniqueness checks should use Names()/Count().
func (r *Registry) Register(c Command) {
	r.cmds[c.Name] = c
}

// Has reports whether a command with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.cmds[name]
	return ok
}

// Names returns the sorted command names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.cmds))
	for n := range r.cmds {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Commands returns the registered commands sorted by name.
func (r *Registry) Commands() []Command {
	cmds := make([]Command, 0, len(r.cmds))
	for _, c := range r.cmds {
		cmds = append(cmds, c)
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return cmds
}

// Count returns the number of registered commands.
func (r *Registry) Count() int { return len(r.cmds) }
