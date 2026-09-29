package jwt

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const minSecretLen = 32

type keyring struct {
	keys      map[string][]byte // key -> secret
	activeKID string
}

func parseKeys(raw, activeKID string) (keyring, error) {
	ring := keyring{
		keys:      make(map[string][]byte),
		activeKID: activeKID,
	}

	for _, entry := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		kid, encoded, ok := strings.Cut(entry, ":")
		if !ok {
			return keyring{}, fmt.Errorf("entry %q is not in the expected 'kid:secret' format", entry)
		}

		kid = strings.TrimSpace(kid)
		if kid == "" {
			return keyring{}, fmt.Errorf("entry %q is missing kid", entry)
		}

		// Secret trong file ở dạng base64
		secret, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil {
			return keyring{}, fmt.Errorf("kid %q: secret not in base64: %w", kid, err)
		}
		if len(secret) < minSecretLen {
			return keyring{}, fmt.Errorf("kid %q: secret is %d bytes long, minimum length must be %d", kid, len(secret), minSecretLen)
		}

		ring.keys[kid] = secret
	}

	if len(ring.keys) == 0 {
		return keyring{}, fmt.Errorf("empty keyring")
	}

	if _, ok := ring.keys[activeKID]; !ok {
		return keyring{}, fmt.Errorf("active kid %q not found in keyring", activeKID)
	}

	return ring, nil
}

// active trả kid đang dùng để ký và secret của nó
func (k *keyring) active() (string, []byte) {
	return k.activeKID, k.keys[k.activeKID]
}

func (k *keyring) lookup(kid string) ([]byte, bool) {
	secret, ok := k.keys[kid]
	return secret, ok
}
