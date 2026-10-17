package main

// MD4 (RFC 1320), só para conferir a senha contra o hash NT que o Samba guarda.
// Não é usado para nada além disso.

import (
	"encoding/binary"
	"math/bits"
	"unicode/utf16"
)

func md4(msg []byte) [16]byte {
	a, b, c, d := uint32(0x67452301), uint32(0xefcdab89), uint32(0x98badcfe), uint32(0x10325476)
	ml := uint64(len(msg)) * 8
	msg = append(append([]byte{}, msg...), 0x80)
	for len(msg)%64 != 56 {
		msg = append(msg, 0)
	}
	var lb [8]byte
	binary.LittleEndian.PutUint64(lb[:], ml)
	msg = append(msg, lb[:]...)
	f := func(x, y, z uint32) uint32 { return x&y | ^x&z }
	g := func(x, y, z uint32) uint32 { return x&y | x&z | y&z }
	h := func(x, y, z uint32) uint32 { return x ^ y ^ z }
	for o := 0; o < len(msg); o += 64 {
		var X [16]uint32
		for i := range X {
			X[i] = binary.LittleEndian.Uint32(msg[o+4*i:])
		}
		aa, bb, cc, dd := a, b, c, d
		for _, i := range []int{0, 4, 8, 12} {
			a = bits.RotateLeft32(a+f(b, c, d)+X[i], 3)
			d = bits.RotateLeft32(d+f(a, b, c)+X[i+1], 7)
			c = bits.RotateLeft32(c+f(d, a, b)+X[i+2], 11)
			b = bits.RotateLeft32(b+f(c, d, a)+X[i+3], 19)
		}
		for _, i := range []int{0, 1, 2, 3} {
			a = bits.RotateLeft32(a+g(b, c, d)+X[i]+0x5a827999, 3)
			d = bits.RotateLeft32(d+g(a, b, c)+X[i+4]+0x5a827999, 5)
			c = bits.RotateLeft32(c+g(d, a, b)+X[i+8]+0x5a827999, 9)
			b = bits.RotateLeft32(b+g(c, d, a)+X[i+12]+0x5a827999, 13)
		}
		for _, i := range []int{0, 2, 1, 3} {
			a = bits.RotateLeft32(a+h(b, c, d)+X[i]+0x6ed9eba1, 3)
			d = bits.RotateLeft32(d+h(a, b, c)+X[i+8]+0x6ed9eba1, 9)
			c = bits.RotateLeft32(c+h(d, a, b)+X[i+4]+0x6ed9eba1, 11)
			b = bits.RotateLeft32(b+h(c, d, a)+X[i+12]+0x6ed9eba1, 15)
		}
		a, b, c, d = a+aa, b+bb, c+cc, d+dd
	}
	var out [16]byte
	binary.LittleEndian.PutUint32(out[0:], a)
	binary.LittleEndian.PutUint32(out[4:], b)
	binary.LittleEndian.PutUint32(out[8:], c)
	binary.LittleEndian.PutUint32(out[12:], d)
	return out
}

// ntHash: MD4 da senha em UTF-16LE (o formato que o Samba guarda).
func ntHash(pw string) [16]byte {
	u := utf16.Encode([]rune(pw))
	b := make([]byte, len(u)*2)
	for i, r := range u {
		binary.LittleEndian.PutUint16(b[2*i:], r)
	}
	return md4(b)
}
