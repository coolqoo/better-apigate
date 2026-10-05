# Single Sign-On (SSO)

better-apigate supports Single Sign-On through OAuth 2.0 and OpenID Connect.

---

## Overview

SSO allows users to sign in using their existing identity provider:

- Google Workspace
- GitHub
- Azure AD
- Okta
- Any OIDC provider

---

## Supported Providers

### Google

```bash
better-apigate settings set oauth.google.enabled true
better-apigate settings set oauth.google.client_id "xxx.googleusercontent.com"
better-apigate settings set oauth.google.client_secret "xxx" --encrypted
```

### GitHub

```bash
better-apigate settings set oauth.github.enabled true
better-apigate settings set oauth.github.client_id "xxx"
better-apigate settings set oauth.github.client_secret "xxx" --encrypted
```

### Generic OIDC

For enterprise IdPs (Okta, Azure AD, etc.):

```bash
better-apigate settings set oauth.oidc.enabled true
better-apigate settings set oauth.oidc.issuer_url "https://your-idp.com"
better-apigate settings set oauth.oidc.client_id "xxx"
better-apigate settings set oauth.oidc.client_secret "xxx" --encrypted
```

---

## Configuration

See [[OAuth]] for detailed setup instructions.

---

## User Linking

SSO identities are linked to better-apigate users:

- By email (automatic if `oauth.auto_link_by_email=true`)
- Manually via account settings

---

## See Also

- [[OAuth]] - OAuth configuration
- [[Authentication]] - Auth overview
- [[Security]] - Security features
