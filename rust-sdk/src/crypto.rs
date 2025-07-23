//! Cryptographic utilities for the go-server Rust SDK
//!
//! This module provides AES-GCM encryption and decryption functionality
//! compatible with the Go implementation.

use aes_gcm::{Aes256Gcm, Key, Nonce, KeyInit};
use aes_gcm::aead::{Aead, generic_array::GenericArray};
use base64::{Engine as _, engine::general_purpose};
use sha2::{Sha256, Digest};
use serde_json::Value;
use crate::error::{Result, SdkError};

/// Encrypts data using AES-GCM with a deterministic IV derived from the key
/// 
/// This function is compatible with the Go implementation's `encryptData` function.
/// 
/// # Arguments
/// 
/// * `data` - The data to encrypt (will be JSON serialized)
/// * `key` - The encryption key
/// 
/// # Returns
/// 
/// Base64-encoded encrypted data
pub fn encrypt_data(data: &Value, key: &str) -> Result<String> {
    // Serialize data to JSON
    let data_bytes = serde_json::to_vec(data)?;
    
    // Hash the key using SHA-256
    let mut hasher = Sha256::new();
    hasher.update(key.as_bytes());
    let key_hash = hasher.finalize();
    
    // Create AES-256-GCM cipher
    let cipher_key = Key::<Aes256Gcm>::from_slice(&key_hash);
    let cipher = Aes256Gcm::new(cipher_key);
    
    // Generate deterministic IV from key (first 12 bytes of SHA-256 hash)
    let mut iv_hasher = Sha256::new();
    iv_hasher.update(key.as_bytes());
    let iv_hash = iv_hasher.finalize();
    let nonce = Nonce::from_slice(&iv_hash[..12]);
    
    // Encrypt the data
    let ciphertext = cipher.encrypt(nonce, data_bytes.as_ref())
        .map_err(|e| SdkError::Crypto(format!("Encryption failed: {:?}", e)))?;
    
    // Return base64-encoded result
    Ok(general_purpose::STANDARD.encode(ciphertext))
}

/// Encrypts a key using the salt as AES key
/// 
/// This function is compatible with the Go implementation's `saltKey` function.
/// 
/// # Arguments
/// 
/// * `key` - The key to encrypt
/// * `salt` - The salt value used as encryption key
/// 
/// # Returns
/// 
/// Base64-encoded encrypted key
pub fn salt_key(key: &str, salt: i32) -> Result<String> {
    // Use salt to generate SHA-256 hash as AES key
    let salt_str = salt.to_string();
    let mut hasher = Sha256::new();
    hasher.update(salt_str.as_bytes());
    let salt_hash = hasher.finalize();
    
    // Create AES-256-GCM cipher
    let cipher_key = Key::<Aes256Gcm>::from_slice(&salt_hash);
    let cipher = Aes256Gcm::new(cipher_key);
    
    // Generate deterministic IV from salt (first 12 bytes of SHA-256 hash)
    let mut iv_hasher = Sha256::new();
    iv_hasher.update(salt_str.as_bytes());
    let iv_hash = iv_hasher.finalize();
    let nonce = Nonce::from_slice(&iv_hash[..12]);
    
    // Encrypt the key
    let key_bytes = key.as_bytes();
    let ciphertext = cipher.encrypt(nonce, key_bytes)
        .map_err(|e| SdkError::Crypto(format!("Key encryption failed: {:?}", e)))?;
    
    // Return base64-encoded result
    Ok(general_purpose::STANDARD.encode(ciphertext))
}

/// Decrypts data using AES-GCM with a deterministic IV derived from the key
/// 
/// This function is compatible with the Go implementation's `decryptData` function.
/// 
/// # Arguments
/// 
/// * `encrypted_data` - Base64-encoded encrypted data
/// * `key` - The decryption key
/// 
/// # Returns
/// 
/// Decrypted JSON value
pub fn decrypt_data(encrypted_data: &str, key: &str) -> Result<Value> {
    // Base64 decode
    let ciphertext = general_purpose::STANDARD.decode(encrypted_data)?;
    
    // Hash the key using SHA-256
    let mut hasher = Sha256::new();
    hasher.update(key.as_bytes());
    let key_hash = hasher.finalize();
    
    // Create AES-256-GCM cipher
    let cipher_key = Key::<Aes256Gcm>::from_slice(&key_hash);
    let cipher = Aes256Gcm::new(cipher_key);
    
    // Generate deterministic IV from key (first 12 bytes of SHA-256 hash)
    let mut iv_hasher = Sha256::new();
    iv_hasher.update(key.as_bytes());
    let iv_hash = iv_hasher.finalize();
    let nonce = Nonce::from_slice(&iv_hash[..12]);
    
    // Decrypt the data
    let plaintext = cipher.decrypt(nonce, ciphertext.as_ref())
        .map_err(|e| SdkError::Crypto(format!("Decryption failed: {:?}", e)))?;
    
    // Parse JSON
    let value: Value = serde_json::from_slice(&plaintext)?;
    Ok(value)
}

/// Decrypts a salted key
/// 
/// This function is compatible with the Go implementation's `unsaltKey` function.
/// 
/// # Arguments
/// 
/// * `salted_key` - Base64-encoded encrypted key
/// * `salt` - The salt value used for encryption
/// 
/// # Returns
/// 
/// Original decrypted key
pub fn unsalt_key(salted_key: &str, salt: i32) -> Result<String> {
    // Base64 decode
    let ciphertext = general_purpose::STANDARD.decode(salted_key)?;
    
    // Use salt to generate SHA-256 hash as AES key
    let salt_str = salt.to_string();
    let mut hasher = Sha256::new();
    hasher.update(salt_str.as_bytes());
    let salt_hash = hasher.finalize();
    
    // Create AES-256-GCM cipher
    let cipher_key = Key::<Aes256Gcm>::from_slice(&salt_hash);
    let cipher = Aes256Gcm::new(cipher_key);
    
    // Generate deterministic IV from salt (first 12 bytes of SHA-256 hash)
    let mut iv_hasher = Sha256::new();
    iv_hasher.update(salt_str.as_bytes());
    let iv_hash = iv_hasher.finalize();
    let nonce = Nonce::from_slice(&iv_hash[..12]);
    
    // Decrypt the key
    let plaintext = cipher.decrypt(nonce, ciphertext.as_ref())
        .map_err(|e| SdkError::Crypto(format!("Key decryption failed: {:?}", e)))?;
    
    // Convert to string
    String::from_utf8(plaintext)
        .map_err(|e| SdkError::Crypto(format!("Invalid UTF-8 in decrypted key: {}", e)))
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    
    #[test]
    fn test_encrypt_decrypt_data() {
        let data = json!({
            "message": "Hello, World!",
            "number": 42
        });
        let key = "test-key-123";
        
        let encrypted = encrypt_data(&data, key).unwrap();
        let decrypted = decrypt_data(&encrypted, key).unwrap();
        
        assert_eq!(data, decrypted);
    }
    
    #[test]
    fn test_salt_unsalt_key() {
        let key = "my-secret-key";
        let salt = 123456;
        
        let salted = salt_key(key, salt).unwrap();
        let unsalted = unsalt_key(&salted, salt).unwrap();
        
        assert_eq!(key, unsalted);
    }
    
    #[test]
    fn test_deterministic_encryption() {
        let data = json!({"test": "value"});
        let key = "consistent-key";
        
        let encrypted1 = encrypt_data(&data, key).unwrap();
        let encrypted2 = encrypt_data(&data, key).unwrap();
        
        // Should be deterministic
        assert_eq!(encrypted1, encrypted2);
    }
}