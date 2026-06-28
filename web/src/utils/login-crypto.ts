import apiUser from '@/api/modules/user'

export const LOGIN_CRYPTO_ACTION = {
  login: 'login',
  centerEditPassword: 'center.edit_password',
  centerEditInfo: 'center.edit_info',
} as const

interface LoginCryptoChallenge {
  keyId: string
  publicKey: string
  challengeId: string
  nonce: string
  algorithm: string
}

interface EncryptPasswordPayload {
  username: string
  ua: string
  action: string
  password?: string
  oldPassword?: string
  newPassword?: string
}

interface EncryptedPasswordPayload {
  passwordCipher: string
  keyId: string
  challengeId: string
}

export async function encryptPasswordPayload(
  payload: EncryptPasswordPayload,
): Promise<EncryptedPasswordPayload> {
  if (!payload.ua) {
    throw new Error('client fingerprint is missing')
  }
  const challenge = (await apiUser.getLoginCrypto({
    username: payload.username,
    ua: payload.ua,
    action: payload.action,
  })) as unknown as LoginCryptoChallenge

  if (challenge.algorithm !== 'RSA-OAEP-SHA256') {
    throw new Error('unsupported login crypto algorithm')
  }

  const key = await importPublicKey(challenge.publicKey)
  const plain = JSON.stringify({
    username: payload.username,
    password: payload.password,
    oldPassword: payload.oldPassword,
    newPassword: payload.newPassword,
    challengeId: challenge.challengeId,
    nonce: challenge.nonce,
    timestamp: Date.now(),
    ua: payload.ua,
    action: payload.action,
  })
  const cipher = await crypto.subtle.encrypt(
    { name: 'RSA-OAEP' },
    key,
    new TextEncoder().encode(plain),
  )

  return {
    passwordCipher: arrayBufferToBase64(cipher),
    keyId: challenge.keyId,
    challengeId: challenge.challengeId,
  }
}

async function importPublicKey(publicKeyPem: string): Promise<CryptoKey> {
  const binary = atob(
    publicKeyPem
      .replace('-----BEGIN PUBLIC KEY-----', '')
      .replace('-----END PUBLIC KEY-----', '')
      .replace(/\s/g, ''),
  )
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return crypto.subtle.importKey(
    'spki',
    bytes.buffer,
    {
      name: 'RSA-OAEP',
      hash: 'SHA-256',
    },
    false,
    ['encrypt'],
  )
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}
