---
name: weather
description: Get current weather and forecasts (no API key required).
homepage: https://wttr.in/:help
metadata: {"openclaw":{"emoji":"🌤️","requires":{"bins":["curl"]}}}
---

# Weather

Two free services, no API keys needed.

## wttr.in (primary)

Quick one-liner:
```bash
curl -s --connect-timeout 5 --max-time 10 "wttr.in/London?format=3"
# Output: London: ⛅️ +8°C
```

Compact format:
```bash
curl -s --connect-timeout 5 --max-time 10 "wttr.in/London?format=%l:+%c+%t+%h+%w"
# Output: London: ⛅️ +8°C 71% ↙5km/h
```

Full forecast:
```bash
curl -s --connect-timeout 5 --max-time 10 "wttr.in/London?T"
```

Format codes: `%c` condition · `%t` temp · `%h` humidity · `%w` wind · `%l` location · `%m` moon

Tips:
- URL-encode spaces: `wttr.in/New+York`
- Airport codes: `wttr.in/JFK`
- Units: `?m` (metric) `?u` (USCS)
- Today only: `?1` · Current only: `?0`
- PNG: `curl -s --connect-timeout 5 --max-time 10 "wttr.in/Berlin.png" -o /tmp/weather.png`

**Windows**: Always use `https://` prefix and escape special chars:
```cmd
curl -s --connect-timeout 5 --max-time 10 https://wttr.in/London?format=3
```
For Chinese cities, use pinyin: `wttr.in/Xian` for 西安

If `wttr.in` is slow or times out:
- Do one short-timeout request first instead of retrying long-running commands.
- For "today" questions, prefer compact outputs like `?format=3` or `?format=4`.
- If the first request times out, switch to Open-Meteo instead of retrying the same source repeatedly.

## Open-Meteo (fallback, JSON)

Free, no key, good for programmatic use:
```bash
curl -s --connect-timeout 5 --max-time 10 "https://api.open-meteo.com/v1/forecast?latitude=51.5&longitude=-0.12&current_weather=true"
```

Find coordinates for a city, then query. Returns JSON with temp, windspeed, weathercode.

Docs: https://open-meteo.com/en/docs
