package rtk

// FilterFunc is the signature for all RTK filters
type FilterFunc func(content string) string

// Filter represents a registered filter with name and function
type Filter struct {
	Name string
	Func FilterFunc
}

// filters is the global registry of all filters
var filters []Filter

// filterMap provides O(1) lookup by name
var filterMap map[string]FilterFunc

// RegisterFilter adds a filter to the registry
func RegisterFilter(name string, fn FilterFunc) {
	if filterMap == nil {
		filterMap = make(map[string]FilterFunc)
	}
	filters = append(filters, Filter{Name: name, Func: fn})
	filterMap[name] = fn
}

// GetFilter returns a filter by name, or nil if not found
func GetFilter(name string) FilterFunc {
	if filterMap == nil {
		return nil
	}
	return filterMap[name]
}

// GetAliases returns common tool name aliases
// rg -> grep, fd -> find, etc.
func GetAliases() map[string]string {
	return map[string]string{
		"rg":  "grep",
		"fd":  "find",
		"exa": "ls",
	}
}

// ResolveFilter resolves a filter name, including aliases
func ResolveFilter(name string) FilterFunc {
	// Direct lookup
	if fn := GetFilter(name); fn != nil {
		return fn
	}
	// Check aliases
	if alias, ok := GetAliases()[name]; ok {
		return GetFilter(alias)
	}
	return nil
}

func init() {
	// Register all filters
	RegisterFilter("gitdiff", GitDiffFilter)
	RegisterFilter("gitstatus", GitStatusFilter)
	RegisterFilter("grep", GrepFilter)
	RegisterFilter("find", FindFilter)
	RegisterFilter("ls", LSFilter)
	RegisterFilter("tree", TreeFilter)
	RegisterFilter("buildoutput", BuildOutputFilter)
	RegisterFilter("deduplog", DedupLogFilter)
	RegisterFilter("smarttruncate", SmartTruncateFilter)
	RegisterFilter("readnumbered", ReadNumberedFilter)
	RegisterFilter("searchlist", SearchListFilter)
}
