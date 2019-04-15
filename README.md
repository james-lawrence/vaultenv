# vaultenv
an opinionated and simple secrets to environment variable tool

### the rules
- obey standard vault environment variable settings. (VAULT_ADDR, etc)
- all secrets must be valid environment key/value pairs. no additional parsing or conversation is done.
- kv pairs are merged left to right. left most kv pairs are the environment of vaultenv itself.
- always uses the latest version of a secret.

```bash
# given the following secrets
# runtime environment:
# PATH=/usr/bin
# FOO=bar1
# secret/key1:
# FOO=bar2
# BIZZ=BAZZ
# secret/key2:
# FOO=bar3
# HELLO=world
vaultenv secret/key1 secret/key2
# output:
# PATH=/usr/bin
# FOO=bar3
# BIZZ=BAZZ
# HELLO=world
```
