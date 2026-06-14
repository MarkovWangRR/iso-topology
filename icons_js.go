//go:build js

package isotopo

// inlineLocalIcon (js/wasm) is a deliberate no-op: a browser wasm module has no
// filesystem, so a local-file or file:// icon path can't be read. Rather than
// pull the os/syscall-fs surface into the wasm binary (and silently fail at
// runtime), we drop that dependency entirely and pass the reference through
// unchanged — degrading to an unresolved href instead of crashing.
//
// iso:// built-ins, data: URIs and http(s) URLs all render fine in the browser
// and need no inlining; if a host app wants local icons in the wasm path, it
// should inline them to data: URIs before handing the DSL to the module.
func inlineLocalIcon(icon string) string { return icon }
