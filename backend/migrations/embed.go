// Package migrations menanam berkas SQL golang-migrate ke dalam binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
