package core

import "github.com/akzj/go-quickjs/pkg/value"

// Scope represents a lexical scope for variable management
type Scope struct {
	Parent    *Scope
	Variables  map[string]int // name -> local index
	Level      int            // scope nesting level (0 = global)
}

// NewScope creates a new scope
func NewScope(parent *Scope) *Scope {
	level := 0
	if parent != nil {
		level = parent.Level + 1
	}
	return &Scope{
		Parent:   parent,
		Variables: make(map[string]int),
		Level:    level,
	}
}

// Lookup looks up a variable in this scope or parent scopes
// Returns (index, found)
func (s *Scope) Lookup(name string) (int, bool) {
	if idx, ok := s.Variables[name]; ok {
		return idx, true
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}
	return 0, false
}

// Register registers a new variable in this scope, returns its index
func (s *Scope) Register(name string) int {
	if idx, ok := s.Variables[name]; ok {
		return idx
	}
	idx := len(s.Variables)
	s.Variables[name] = idx
	return idx
}

// LocalStorage holds local variable values (attached to frame)
type LocalStorage struct {
	Values []value.JSValue
}

// NewLocalStorage creates storage for given var count
func NewLocalStorage(varCount int) *LocalStorage {
	return &LocalStorage{
		Values: make([]value.JSValue, varCount),
	}
}

// Get returns local variable value
func (ls *LocalStorage) Get(idx int) value.JSValue {
	if idx < len(ls.Values) {
		return ls.Values[idx]
	}
	return value.Undefined()
}

// Set sets local variable value, expanding if needed
func (ls *LocalStorage) Set(idx int, v value.JSValue) {
	if idx >= len(ls.Values) {
		newValues := make([]value.JSValue, idx+1)
		copy(newValues, ls.Values)
		ls.Values = newValues
	}
	ls.Values[idx] = v
}