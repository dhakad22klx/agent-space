# Telegram

To verify: `GET {api}/bot<token>/getMe`, the call the Bot API provides for
exactly this question.

- https://core.telegram.org/bots/api#getme
- https://core.telegram.org/bots/api#authorizing-your-bot
- https://core.telegram.org/bots/features#botfather

Asked for: the bot token, shaped `<id>:<secret>`.

Worth reporting from the reply: `ok`, and from `result` the `username`, `id`,
`first_name`, `can_join_groups` and `supports_inline_queries`.

The token sits in the URL path, not in a header. Two things follow: escape it
before interpolating, or a mistyped slash changes which method is called; and
never print a URL or an unwrapped transport error, because either one carries
the token.

A refusal arrives as `ok: false` with a `description`. Quote it — Telegram
explains the problem better than a guess would.
