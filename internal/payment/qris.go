package payment

import (
	"fmt"
	"strconv"
	"strings"
)

// crc16CCITT computes the CRC16-CCITT (poly 0x1021, init 0xFFFF) over s, as
// required by the EMVCo/QRIS spec for tag 63.
func crc16CCITT(s string) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range []byte(s) {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// StaticToDynamicQRIS converts a merchant's static QRIS payload into a dynamic
// one that encodes a fixed transaction amount, recomputing the CRC. This is the
// standard "QRIS dinamis" transformation:
//   - Point of Initiation Method (tag 01): "11" (static) -> "12" (dynamic)
//   - Insert Transaction Amount (tag 54) before the Country Code (tag 58 "ID")
//   - Recompute the CRC (tag 63)
//
// amount is in whole IDR (no decimals).
func StaticToDynamicQRIS(static string, amount int64) (string, error) {
	s := strings.TrimSpace(static)
	if s == "" {
		return "", fmt.Errorf("static QRIS payload is empty")
	}
	if amount <= 0 {
		return "", fmt.Errorf("amount must be > 0")
	}

	// Strip the existing CRC tag (63 + len 04 + 4 hex) from the end if present.
	if idx := strings.LastIndex(s, "6304"); idx >= 0 && idx >= len(s)-8 {
		s = s[:idx]
	}

	// Static -> dynamic point-of-initiation method.
	s = strings.Replace(s, "010211", "010212", 1)

	// Build the amount tag (54). Value length is 2-digit, zero-padded.
	amt := strconv.FormatInt(amount, 10)
	tag54 := fmt.Sprintf("54%02d%s", len(amt), amt)

	// Insert the amount before the country-code tag "5802ID".
	anchor := "5802ID"
	i := strings.Index(s, anchor)
	if i < 0 {
		return "", fmt.Errorf("invalid QRIS: country-code tag %q not found", anchor)
	}
	s = s[:i] + tag54 + s[i:]

	// Recompute and append the CRC over the payload including the "6304" tag id.
	s += "6304"
	crc := crc16CCITT(s)
	return s + fmt.Sprintf("%04X", crc), nil
}
