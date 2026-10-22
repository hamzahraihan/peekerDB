//go:build !windows

package saved

func encryptPayloadPlain(plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func decryptPayloadPlain(ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}
