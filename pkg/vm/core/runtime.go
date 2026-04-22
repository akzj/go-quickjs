package core

import (
	"github.com/akzj/go-quickjs/pkg/value"
)

// JSRuntime represents the JavaScript runtime (singleton per VM instance)
// For Stage 1, it's minimal - just enough to run simple bytecode
type JSRuntime struct {
	// Atoms table for string interning
	atoms    map[string]int
	atomNext int
}

// NewRuntime creates a new JavaScript runtime
func NewRuntime() *JSRuntime {
	return &JSRuntime{
		atoms:    make(map[string]int),
		atomNext: 0,
	}
}

// InternString adds a string to the atoms table and returns its index
func (rt *JSRuntime) InternString(s string) int {
	if idx, ok := rt.atoms[s]; ok {
		return idx
	}
	idx := rt.atomNext
	rt.atoms[s] = idx
	rt.atomNext++
	return idx
}

// JSContext represents an execution context
type JSContext struct {
	rt       *JSRuntime
	globalObj *ObjectValue // global object (for builtins)
	builtinFuncs map[string]builtinFunc
}

// NewContext creates a new execution context
func NewContext(rt *JSRuntime) *JSContext {
	ctx := &JSContext{
		rt: rt,
		builtinFuncs: make(map[string]builtinFunc),
	}
	// Register built-in functions
	ctx.registerBuiltins()
	return ctx
}

// RunBytecode executes bytecode and returns the result
func (ctx *JSContext) RunBytecode(bc *Bytecode) value.JSValue {
	vm := &VM{
		ctx:   ctx,
		stack: make([]value.JSValue, 0, 64),
	}
	
	frame := NewFrame(bc)
	vm.frame = frame
	
	result := vm.Run()
	
	if result == nil {
		return value.Undefined()
	}
	return result
}

// ObjectValue represents a JavaScript object (placeholder for Stage 1)
type ObjectValue struct {
	properties map[string]value.JSValue
	prototype  *ObjectValue
}

func NewObject() *ObjectValue {
	return &ObjectValue{
		properties: make(map[string]value.JSValue),
	}
}

func (o *ObjectValue) Get(name string) value.JSValue {
	if v, ok := o.properties[name]; ok {
		return v
	}
	if o.prototype != nil {
		return o.prototype.Get(name)
	}
	return value.Undefined()
}

func (o *ObjectValue) Set(name string, v value.JSValue) {
	o.properties[name] = v
}

// builtinFunc is a built-in JavaScript function
type builtinFunc struct {
	name string
	fn   func(*JSContext, []value.JSValue) value.JSValue
}

// registerBuiltins registers built-in JavaScript functions
func (ctx *JSContext) registerBuiltins() {
	// Stage 1: minimal builtins, just need eval
}

// Eval executes JavaScript source code
func (ctx *JSContext) Eval(source string) value.JSValue {
	return ctx.CompileAndRun(source)
}