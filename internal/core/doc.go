// Package core is ambit-core: schema read and write, the in-memory graph index,
// and validated mutations. Both front doors call this package.
//
// Layout cache, proposal apply, and the scope check are later stages. They are
// not implemented here.
package core
