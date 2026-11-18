package anchoring

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
)

// secp256k1 curve arithmetic on math/big (affine coordinates). This is a
// low-throughput anchoring tool — a handful of signatures per batch — so
// clarity beats speed. Signing is RFC 6979 deterministic (HMAC-SHA256),
// with low-s normalization (EIP-2) and a recovery id, as Ethereum expects.

var (
	secpP, _  = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16)
	secpN, _  = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	secpGx, _ = new(big.Int).SetString("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", 16)
	secpGy, _ = new(big.Int).SetString("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8", 16)
)

type curvePoint struct {
	x, y *big.Int
	inf  bool
}

func pointAdd(p, q curvePoint) curvePoint {
	if p.inf {
		return q
	}
	if q.inf {
		return p
	}
	if p.x.Cmp(q.x) == 0 {
		// Same x: either doubling (same y) or inverse (sum = infinity).
		ysum := new(big.Int).Add(p.y, q.y)
		ysum.Mod(ysum, secpP)
		if ysum.Sign() == 0 {
			return curvePoint{inf: true}
		}
		return pointDouble(p)
	}
	// slope = (qy - py) / (qx - px)
	num := new(big.Int).Sub(q.y, p.y)
	den := new(big.Int).Sub(q.x, p.x)
	den.Mod(den, secpP)
	inv := new(big.Int).ModInverse(den, secpP)
	slope := new(big.Int).Mul(num, inv)
	slope.Mod(slope, secpP)
	// x3 = slope^2 - px - qx
	x3 := new(big.Int).Mul(slope, slope)
	x3.Sub(x3, p.x)
	x3.Sub(x3, q.x)
	x3.Mod(x3, secpP)
	// y3 = slope*(px - x3) - py
	y3 := new(big.Int).Sub(p.x, x3)
	y3.Mul(slope, y3)
	y3.Sub(y3, p.y)
	y3.Mod(y3, secpP)
	return curvePoint{x: x3, y: y3}
}

func pointDouble(p curvePoint) curvePoint {
	if p.inf || p.y.Sign() == 0 {
		return curvePoint{inf: true}
	}
	// slope = 3*x^2 / (2*y)  (a = 0 for secp256k1)
	num := new(big.Int).Mul(p.x, p.x)
	num.Mul(num, big.NewInt(3))
	den := new(big.Int).Lsh(p.y, 1)
	den.Mod(den, secpP)
	inv := new(big.Int).ModInverse(den, secpP)
	slope := new(big.Int).Mul(num, inv)
	slope.Mod(slope, secpP)
	x3 := new(big.Int).Mul(slope, slope)
	x3.Sub(x3, new(big.Int).Lsh(p.x, 1))
	x3.Mod(x3, secpP)
	y3 := new(big.Int).Sub(p.x, x3)
	y3.Mul(slope, y3)
	y3.Sub(y3, p.y)
	y3.Mod(y3, secpP)
	return curvePoint{x: x3, y: y3}
}

func scalarMult(k *big.Int, p curvePoint) curvePoint {
	result := curvePoint{inf: true}
	addend := p
	kk := new(big.Int).Set(k)
	for i := 0; i < kk.BitLen(); i++ {
		if kk.Bit(i) == 1 {
			result = pointAdd(result, addend)
		}
		addend = pointDouble(addend)
	}
	return result
}

// PrivateKey is a 32-byte secp256k1 secret.
type PrivateKey [32]byte

func (k PrivateKey) scalar() *big.Int {
	return new(big.Int).SetBytes(k[:])
}

// PublicKey returns the uncompressed public key point.
func (k PrivateKey) PublicKey() curvePoint {
	return scalarMult(k.scalar(), curvePoint{x: secpGx, y: secpGy})
}

// Address returns the EIP-55 checksummed Ethereum address for the key.
func (k PrivateKey) Address() string {
	pub := k.PublicKey()
	return pointToAddress(pub)
}

func pointToAddress(pub curvePoint) string {
	buf := make([]byte, 64)
	xb := pub.x.Bytes()
	yb := pub.y.Bytes()
	copy(buf[32-len(xb):32], xb)
	copy(buf[64-len(yb):64], yb)
	h := Keccak256(buf)
	addr := h[12:]
	return eip55(addr)
}

func eip55(addr []byte) string {
	const hexdigits = "0123456789abcdef"
	raw := make([]byte, 40)
	for i, b := range addr {
		raw[2*i] = hexdigits[b>>4]
		raw[2*i+1] = hexdigits[b&0x0f]
	}
	hash := Keccak256(raw)
	out := []byte("0x")
	for i, c := range raw {
		if c >= 'a' && c <= 'f' {
			nibble := hash[i/2]
			if i%2 == 0 {
				nibble >>= 4
			}
			if nibble&0x0f >= 8 {
				c = c - 'a' + 'A'
			}
		}
		out = append(out, c)
	}
	return string(out)
}

// rfc6979Nonce generates the deterministic nonce for (priv, hash) per RFC 6979
// with HMAC-SHA256, as used by Ethereum signing.
func rfc6979Nonce(priv *big.Int, hash [32]byte) *big.Int {
	// qlen = 256, rolen = 32.
	x := make([]byte, 32)
	xb := priv.Bytes()
	copy(x[32-len(xb):], xb)
	h1 := make([]byte, 32)
	copy(h1, hash[:])
	// bits2octets(h1): h1 mod n, as 32-byte big-endian.
	h1int := new(big.Int).SetBytes(h1)
	h1int.Mod(h1int, secpN)
	h1b := h1int.Bytes()
	h1 = make([]byte, 32)
	copy(h1[32-len(h1b):], h1b)

	v := make([]byte, 32)
	for i := range v {
		v[i] = 0x01
	}
	k := make([]byte, 32) // zeros

	mac := hmac.New(sha256.New, k)
	mac.Write(v)
	mac.Write([]byte{0x00})
	mac.Write(x)
	mac.Write(h1)
	k = mac.Sum(nil)
	mac = hmac.New(sha256.New, k)
	mac.Write(v)
	v = mac.Sum(nil)

	mac = hmac.New(sha256.New, k)
	mac.Write(v)
	mac.Write([]byte{0x01})
	mac.Write(x)
	mac.Write(h1)
	k = mac.Sum(nil)
	mac = hmac.New(sha256.New, k)
	mac.Write(v)
	v = mac.Sum(nil)

	for {
		mac = hmac.New(sha256.New, k)
		mac.Write(v)
		v = mac.Sum(nil)
		candidate := new(big.Int).SetBytes(v)
		if candidate.Sign() > 0 && candidate.Cmp(secpN) < 0 {
			return candidate
		}
		mac = hmac.New(sha256.New, k)
		mac.Write(v)
		mac.Write([]byte{0x00})
		k = mac.Sum(nil)
		mac = hmac.New(sha256.New, k)
		mac.Write(v)
		v = mac.Sum(nil)
	}
}

// Sign produces an Ethereum ECDSA signature (r, s, recid) over a 32-byte hash.
// s is normalized to the lower half of the curve order (EIP-2).
func (k PrivateKey) Sign(hash [32]byte) (r, s *big.Int, recid int, err error) {
	priv := k.scalar()
	if priv.Sign() <= 0 || priv.Cmp(secpN) >= 0 {
		return nil, nil, 0, errors.New("private key out of range")
	}
	nonce := rfc6979Nonce(priv, hash)
	R := scalarMult(nonce, curvePoint{x: secpGx, y: secpGy})
	r = new(big.Int).Mod(R.x, secpN)
	if r.Sign() == 0 {
		return nil, nil, 0, errors.New("signature r is zero")
	}
	recid = 0
	if R.y.Bit(0) == 1 {
		recid |= 1
	}
	if R.x.Cmp(secpN) >= 0 {
		recid |= 2
	}
	// s = nonce^-1 * (hash + r*priv) mod n
	e := new(big.Int).SetBytes(hash[:])
	s = new(big.Int).Mul(r, priv)
	s.Add(s, e)
	inv := new(big.Int).ModInverse(nonce, secpN)
	s.Mul(s, inv)
	s.Mod(s, secpN)
	if s.Sign() == 0 {
		return nil, nil, 0, errors.New("signature s is zero")
	}
	// Low-s normalization.
	halfN := new(big.Int).Rsh(secpN, 1)
	if s.Cmp(halfN) > 0 {
		s.Sub(secpN, s)
		recid ^= 1
	}
	return r, s, recid, nil
}

// RecoverAddress recovers the signer address from (hash, r, s, recid).
func RecoverAddress(hash [32]byte, r, s *big.Int, recid int) (string, error) {
	if recid < 0 || recid > 3 {
		return "", fmt.Errorf("invalid recovery id %d", recid)
	}
	x := new(big.Int).Set(r)
	if recid&2 != 0 {
		x.Add(x, secpN)
	}
	if x.Cmp(secpP) >= 0 {
		return "", errors.New("recovered x out of range")
	}
	// y = sqrt(x^3 + 7) mod p; p % 4 == 3, so sqrt(a) = a^((p+1)/4).
	y2 := new(big.Int).Mul(x, x)
	y2.Mul(y2, x)
	y2.Add(y2, big.NewInt(7))
	y2.Mod(y2, secpP)
	exp := new(big.Int).Add(secpP, big.NewInt(1))
	exp.Rsh(exp, 2)
	y := new(big.Int).Exp(y2, exp, secpP)
	if int(y.Bit(0)) != recid&1 {
		y.Sub(secpP, y)
	}
	R := curvePoint{x: x, y: y}
	// Q = r^-1 * (s*R - e*G)
	e := new(big.Int).SetBytes(hash[:])
	sR := scalarMult(s, R)
	eG := scalarMult(e, curvePoint{x: secpGx, y: secpGy})
	negEG := curvePoint{x: eG.x, y: new(big.Int).Mod(new(big.Int).Neg(eG.y), secpP), inf: eG.inf}
	sum := pointAdd(sR, negEG)
	rInv := new(big.Int).ModInverse(r, secpN)
	Q := scalarMult(rInv, sum)
	if Q.inf {
		return "", errors.New("recovered point at infinity")
	}
	return pointToAddress(Q), nil
}
