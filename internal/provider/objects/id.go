//nolint:recvcheck
package objects

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalidFormat = errors.New("invalid formatted object ID")

type ID struct {
	prefix string
	uid    uuid.UUID
}

func New(prefix string, uid uuid.UUID) *ID {
	return &ID{
		prefix: prefix,
		uid:    uid,
	}
}

func Parse(in string) (*ID, error) {
	before, after, ok := strings.Cut(in, "_")
	if !ok {
		return nil, ErrInvalidFormat
	}
	b, err := base64.RawURLEncoding.DecodeString(after)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFormat, err)
	}
	if len(b) != len(uuid.Nil) {
		return nil, fmt.Errorf("%w: could not extract uuid", ErrInvalidFormat)
	}
	return New(before, uuid.UUID(b)), nil
}

func (id *ID) UnmarshalJSON(text []byte) error {
	if len(text) > 2 && text[0] == '"' && text[len(text)-1] == '"' {
		return id.UnmarshalText(text[1 : len(text)-1])
	}
	return id.UnmarshalText(text)
}

func (id *ID) UnmarshalText(text []byte) error {
	i, err := Parse(string(text))
	if err != nil {
		return err
	}
	id.uid = i.uid
	id.prefix = i.prefix
	return nil
}

func (id ID) String() string {
	return fmt.Sprintf("%s_%s", id.prefix, base64.RawURLEncoding.EncodeToString(id.uid[:]))
}

func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

func (id ID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

func (id ID) IsZero() bool {
	return id.prefix == "" && id.uid == uuid.Nil
}
