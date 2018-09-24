## Vault utils

### Login

- `Login(roleID, secretID)` - client login with given permissions

### Read

  `Login` method required before

- `Read("secret/read/foo", "foo")` - reads from `secret/read/foo`
value of `foo`


### Encrypt\Decript

[Documentation](https://www.vaultproject.io/docs/secrets/transit/index.html)

`Transit secrets engine` must be enabled by operator command

```
vault secrets enable transit
```

#### Work:

- `Login` method required before


  A client must have permission to write in `transit/*` for creating keys and work with data.
Or operator must create client key and give permission to the client for work in `transit/encrypt/client_key` and `transit/decrypt/client_key`


- `CreateTransitKey(transitKey)` - create encryption key


- `EncryptData(transitKey, data)` - encrypt `data` (`base64`-encoded our data).

    Response contains encrypted data. The client must store this data and `transitKey` encryption key.

- `DecryptData(transitKey, encrypted)`  - decript `encrypted` data with `transitKey`.
Response contains our `base64`-encoded data.