# Image Assets

Place managed outbound image assets in this directory.

The bot only sends images that are registered in `index.json`.

Each asset entry should look like:

```json
{
  "id": "good_night_cat",
  "file": "good_night_cat.jpg",
  "title": "晚安猫咪",
  "description": "适合晚安、收尾、轻松安抚场景",
  "tags": ["晚安", "睡觉", "猫", "可爱"],
  "enabled": true
}
```

Rules:

- `id` must be unique
- `file` can be a relative local file name, an absolute path, or an `http/https` URL
- only `enabled=true` assets can be used by the bot
