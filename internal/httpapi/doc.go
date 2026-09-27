// Package httpapi is the localhost HTTP API served by ambit start: graph reads,
// and later graph writes, the layout cache, and the SSE stream.
//
// Mutations, when they exist, call ambit-core. This package does not validate
// the model a second time.
package httpapi
