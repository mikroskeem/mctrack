package doh

import (
	"net"
)

func dnsError(desc string, timeout, temporary, notFound bool) *net.DNSError {
	return &net.DNSError{
		Err:         desc,
		IsTimeout:   timeout,
		IsTemporary: temporary,
		IsNotFound:  notFound,
	}
}

// ErrFormatError means that the name server was unable to interpret the query.
var ErrFormatError = dnsError("Query format error", false, false, false)

// ErrServerFailure means that The name server was unable to process this query
// due to a problem with the name server.
var ErrServerFailure = dnsError("Server failure", false, false, false)

// ErrNameError means that the domain name referenced in the query does not
// exist.
var ErrNameError = dnsError("Name error", false, false, true)

// ErrNotImplemented means that the name server does not support the requested
// kind of query.
var ErrNotImplemented = dnsError("Not implemented", false, false, false)

// ErrRefused means that The name server refuses to perform the specified
// operation for policy reasons. For example, a name server may not wish to
// provide the information to the particular requester, or a name server may not
// wish to perform a particular operation (e.g., zone transfer) for particular
// data.
var ErrRefused = dnsError("Refused", false, false, false)

var dnsErrors = []error{
	nil,
	ErrFormatError,
	ErrServerFailure,
	ErrNameError,
	ErrNotImplemented,
	ErrRefused,
}

// ErrNotAResponse means that the server responded with a message that isn't a
// response.
var ErrNotAResponse = dnsError("The message the server sent us isn't a response", false, false, false)

// ErrNotIN means that the lookup can only be performed with the DNS class IN
// (e.g. A, AAAA).
var ErrNotIN = dnsError("Class must be IN (Internet) (or ANYCLASS (*), which includes IN)", false, false, false)

// ErrNotStandardQuery means that the server responded with an OPCODE header
// that isn't a standard query, which is the only value currently supported.
var ErrNotStandardQuery = dnsError("Only standard queries are supported", false, false, false)

// ErrTruncated means that the message is truncated, which isn't currently
// supported.
var ErrTruncated = dnsError("Truncated messages aren't supported", false, false, false)

// ErrCorrupted means that the message sent back by the server is either empty,
// incomplete, or corrupted.
var ErrCorrupted = dnsError("The message the server sent is empty, incomplete, or corrupted", false, false, false)

var ErrTimeout = dnsError("Timeout", true, true, false)
