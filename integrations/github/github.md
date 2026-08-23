# GitHub

To verify: `GET {api}/user`, which answers only for a credential GitHub accepts.

- https://docs.github.com/en/rest/users/users#get-the-authenticated-user
- https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api
- https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens

Asked for: the token, and the API base URL (blank keeps `https://api.github.com`;
GitHub Enterprise is `https://<host>/api/v3`).

Send `Authorization: Bearer <token>`, `Accept: application/vnd.github+json` and
`X-GitHub-Api-Version: 2022-11-28`.

Worth reporting from the reply: `login`, `name` and `type` from the body;
`X-OAuth-Scopes` and `X-RateLimit-Remaining` from the headers. Fine-grained
tokens send no scope header, so report scopes only when GitHub sends them.

`401` is a rejected token. `403` is a token GitHub knows but will not act on —
rate limit, or an organisation policy — so it deserves different wording. A
`200` carrying no `login` means the base URL is not the GitHub API, which is an
error rather than a pass.
