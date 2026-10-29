/* ============================================================
   SHA-1 / SHA-256 / SHA-384 / SHA-512（纯 JS 实现）
   ------------------------------------------------------------
   微应用可能被宿主以 http 源加载，此时浏览器不提供 crypto.subtle
   （只有安全上下文才有）。哈希工具不能因此变成不可用，
   所以这里自带实现，结果与 sha1sum/sha256sum 等完全一致。

   常量表不写死字面量，而是按 FIPS 180-4 的定义从素数算出：
     H_256 = 前 8 个素数平方根小数部分的高 32 位
     K_256 = 前 64 个素数立方根小数部分的高 32 位
     H_512 = 前 8 个素数平方根小数部分的高 64 位
     K_512 = 前 80 个素数立方根小数部分的高 64 位
   这样既不会抄错常量，也不用维护一大串魔数。
   ============================================================ */

const M32 = 0xffffffffn
const M64 = (1n << 64n) - 1n

/** 前 n 个素数（筛法，n=80 只需要算到 409） */
function firstPrimes(n) {
  const out = []
  for (let x = 2; out.length < n; x++) {
    let prime = true
    for (const p of out) {
      if (p * p > x) break
      if (x % p === 0) {
        prime = false
        break
      }
    }
    if (prime) out.push(x)
  }
  return out
}

/** BigInt 整数平方根：floor(sqrt(n)) */
function isqrt(n) {
  if (n < 2n) return n
  let x = 1n << BigInt(Math.ceil(n.toString(2).length / 2) + 1)
  for (;;) {
    const y = (x + n / x) >> 1n
    if (y >= x) break
    x = y
  }
  while (x * x > n) x -= 1n
  while ((x + 1n) * (x + 1n) <= n) x += 1n
  return x
}

/** BigInt 整数立方根：floor(cbrt(n)) */
function icbrt(n) {
  if (n < 2n) return n
  let x = 1n << BigInt(Math.ceil(n.toString(2).length / 3) + 1)
  for (;;) {
    const y = (2n * x + n / (x * x)) / 3n
    if (y >= x) break
    x = y
  }
  while (x * x * x > n) x -= 1n
  while ((x + 1n) * (x + 1n) * (x + 1n) <= n) x += 1n
  return x
}

const PRIMES = firstPrimes(80)
const fracSqrt32 = PRIMES.slice(0, 8).map((p) => Number(isqrt(BigInt(p) << 64n) & M32) >>> 0)
const fracSqrt64 = PRIMES.slice(0, 8).map((p) => isqrt(BigInt(p) << 128n) & M64)
const K256 = PRIMES.slice(0, 64).map((p) => Number(icbrt(BigInt(p) << 96n) & M32) >>> 0)
const K512 = PRIMES.map((p) => icbrt(BigInt(p) << 192n) & M64)

const H256 = fracSqrt32
// SHA-512 用前 8 个素数的平方根小数部分，SHA-384 用第 9~16 个，
// 输出也截断到 48 字节——两者算法相同，只有初始值和截断长度不同。
const H512 = fracSqrt64
const H384 = PRIMES.slice(8, 16).map((p) => isqrt(BigInt(p) << 128n) & M64)

const rotr = (x, n) => ((x >>> n) | (x << (32 - n))) >>> 0
const rotr64 = (x, n) => ((x >> n) | (x << (64n - n))) & M64

const HEX = []
for (let i = 0; i < 256; i++) HEX.push(i.toString(16).padStart(2, '0'))

// 64 位字按高/低 32 位拆成 8 个字节输出
function toHex(words) {
  let s = ''
  for (const w of words) {
    const hi = Number((w >> 32n) & 0xffffffffn) >>> 0
    const lo = Number(w & 0xffffffffn) >>> 0
    s += HEX[(hi >>> 24) & 0xff] + HEX[(hi >>> 16) & 0xff] + HEX[(hi >>> 8) & 0xff] + HEX[hi & 0xff]
    s += HEX[(lo >>> 24) & 0xff] + HEX[(lo >>> 16) & 0xff] + HEX[(lo >>> 8) & 0xff] + HEX[lo & 0xff]
  }
  return s
}

function toHex32(words) {
  let s = ''
  for (const w of words) {
    s += HEX[(w >>> 24) & 0xff] + HEX[(w >>> 16) & 0xff] + HEX[(w >>> 8) & 0xff] + HEX[w & 0xff]
  }
  return s
}

/** Merkle–Damgård 填充：0x80 + 补零 + 大端消息位长 */
function pad(bytes, blockLen, lenBytes) {
  const lenBits = BigInt(bytes.length) * 8n
  const blocks = Math.ceil((bytes.length + 1 + lenBytes) / blockLen)
  const out = new Uint8Array(blocks * blockLen)
  out.set(bytes)
  out[bytes.length] = 0x80
  const tail = out.length - 1
  for (let i = 0; i < lenBytes; i++) {
    out[tail - i] = Number((lenBits >> BigInt(8 * i)) & 0xffn)
  }
  return out
}

const utf8 = (s) => new TextEncoder().encode(s)

function sha1(bytes) {
  const p = pad(bytes, 64, 8)
  const w = new Uint32Array(80)
  let h0 = 0x67452301, h1 = 0xefcdab89, h2 = 0x98badcfe, h3 = 0x10325476, h4 = 0xc3d2e1f0

  for (let off = 0; off < p.length; off += 64) {
    for (let i = 0; i < 16; i++) {
      w[i] = (p[off + i * 4] << 24) | (p[off + i * 4 + 1] << 16) | (p[off + i * 4 + 2] << 8) | p[off + i * 4 + 3]
    }
    for (let i = 16; i < 80; i++) {
      const v = w[i - 3] ^ w[i - 8] ^ w[i - 14] ^ w[i - 16]
      w[i] = ((v << 1) | (v >>> 31)) >>> 0
    }
    let a = h0, b = h1, c = h2, d = h3, e = h4
    for (let i = 0; i < 80; i++) {
      let f, k
      if (i < 20) {
        f = (b & c) | (~b & d)
        k = 0x5a827999
      } else if (i < 40) {
        f = b ^ c ^ d
        k = 0x6ed9eba1
      } else if (i < 60) {
        f = (b & c) | (b & d) | (c & d)
        k = 0x8f1bbcdc
      } else {
        f = b ^ c ^ d
        k = 0xca62c1d6
      }
      const t = (((a << 5) | (a >>> 27)) + f + e + k + w[i]) >>> 0
      e = d
      d = c
      c = ((b << 30) | (b >>> 2)) >>> 0
      b = a
      a = t
    }
    h0 = (h0 + a) >>> 0
    h1 = (h1 + b) >>> 0
    h2 = (h2 + c) >>> 0
    h3 = (h3 + d) >>> 0
    h4 = (h4 + e) >>> 0
  }
  return toHex32([h0, h1, h2, h3, h4])
}

function sha256(bytes) {
  const p = pad(bytes, 64, 8)
  const w = new Uint32Array(64)
  const H = H256.slice()

  for (let off = 0; off < p.length; off += 64) {
    for (let i = 0; i < 16; i++) {
      w[i] = (p[off + i * 4] << 24) | (p[off + i * 4 + 1] << 16) | (p[off + i * 4 + 2] << 8) | p[off + i * 4 + 3]
    }
    for (let i = 16; i < 64; i++) {
      const s0 = (rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >>> 3)) >>> 0
      const s1 = (rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >>> 10)) >>> 0
      w[i] = (w[i - 16] + s0 + w[i - 7] + s1) >>> 0
    }
    let [a, b, c, d, e, f, g, h] = H
    for (let i = 0; i < 64; i++) {
      const S1 = (rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)) >>> 0
      const ch = ((e & f) ^ (~e & g)) >>> 0
      const t1 = (h + S1 + ch + K256[i] + w[i]) >>> 0
      const S0 = (rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)) >>> 0
      const mj = ((a & b) ^ (a & c) ^ (b & c)) >>> 0
      const t2 = (S0 + mj) >>> 0
      h = g
      g = f
      f = e
      e = (d + t1) >>> 0
      d = c
      c = b
      b = a
      a = (t1 + t2) >>> 0
    }
    H[0] = (H[0] + a) >>> 0
    H[1] = (H[1] + b) >>> 0
    H[2] = (H[2] + c) >>> 0
    H[3] = (H[3] + d) >>> 0
    H[4] = (H[4] + e) >>> 0
    H[5] = (H[5] + f) >>> 0
    H[6] = (H[6] + g) >>> 0
    H[7] = (H[7] + h) >>> 0
  }
  return toHex32(H)
}

function sha512core(bytes, H0, wordCount) {
  const p = pad(bytes, 128, 16)
  const w = new Array(80)
  let [a, b, c, d, e, f, g, h] = H0

  for (let off = 0; off < p.length; off += 128) {
    for (let i = 0; i < 16; i++) {
      let v = 0n
      for (let j = 0; j < 8; j++) v = (v << 8n) | BigInt(p[off + i * 8 + j])
      w[i] = v
    }
    for (let i = 16; i < 80; i++) {
      const s0 = rotr64(w[i - 15], 1n) ^ rotr64(w[i - 15], 8n) ^ (w[i - 15] >> 7n)
      const s1 = rotr64(w[i - 2], 19n) ^ rotr64(w[i - 2], 61n) ^ (w[i - 2] >> 6n)
      w[i] = (w[i - 16] + s0 + w[i - 7] + s1) & M64
    }
    let A = a, B = b, C = c, D = d, E = e, F = f, G = g, H = h
    for (let i = 0; i < 80; i++) {
      const S1 = rotr64(E, 14n) ^ rotr64(E, 18n) ^ rotr64(E, 41n)
      const ch = (E & F) ^ (~E & G)
      const t1 = (H + S1 + ch + K512[i] + w[i]) & M64
      const S0 = rotr64(A, 28n) ^ rotr64(A, 34n) ^ rotr64(A, 39n)
      const mj = (A & B) ^ (A & C) ^ (B & C)
      const t2 = (S0 + mj) & M64
      H = G
      G = F
      F = E
      E = (D + t1) & M64
      D = C
      C = B
      B = A
      A = (t1 + t2) & M64
    }
    a = (a + A) & M64
    b = (b + B) & M64
    c = (c + C) & M64
    d = (d + D) & M64
    e = (e + E) & M64
    f = (f + F) & M64
    g = (g + G) & M64
    h = (h + H) & M64
  }
  return toHex([a, b, c, d, e, f, g, h].slice(0, wordCount))
}

export const sha1Hex = (text) => sha1(utf8(text))
export const sha256Hex = (text) => sha256(utf8(text))
export const sha384Hex = (text) => sha512core(utf8(text), H384, 6)
export const sha512Hex = (text) => sha512core(utf8(text), H512, 8)

export const DIGESTS = [
  ['SHA-1', sha1Hex],
  ['SHA-256', sha256Hex],
  ['SHA-384', sha384Hex],
  ['SHA-512', sha512Hex],
]
