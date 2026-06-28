// Package idcodec provides reversible public ID encoding for numeric IDs.
//
// The codec is intended to hide raw incrementing IDs in URLs, order numbers,
// and third-party callback payloads while keeping BIGINT primary keys in the
// database. A public ID is not an authorization credential; callers must still
// enforce user, workspace, and resource permissions after decoding.
package idcodec
