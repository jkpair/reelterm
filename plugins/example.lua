-- reelterm example plugin
--
-- Copy to ~/.config/reelterm/plugins/ and load it with `reelterm -x example`, or
-- add it to AUTO_LOADED_EXTENSIONS in the config to load it on every start.
--
-- API (global `yt` table):
--   yt.menu_item(label, icon, fn)      add a main-menu entry; fn() runs when chosen
--   yt.video_action(label, icon, fn)   add a video action; fn(video) gets the selected video
--   yt.on(event, fn)                   hooks: "play", "download", "search"; fn(video)
--   yt.ytdlp(url[, limit])             -> list of videos (or nil, err)
--   yt.search(query[, limit])          -> list of videos; supports :filters
--   yt.mix_url(video_id)               -> YouTube mix URL for a video
--   yt.show_list(title, videos|url)    open a list (with previews) when fn returns
--   yt.play(url[, {audio=true}])       play in the configured player
--   yt.download(url[, {audio=true}])   queue a background download
--   yt.notify(msg)                     show a status message
--   yt.open(url)                       open in the browser
--   yt.shell(cmd)                      -> output of a shell command
--   yt.config(key)                     -> raw config value, e.g. yt.config("PLAYER")
--
-- A video is a table: id, title, url, channel, channel_url, thumbnail,
-- description, duration, views, duration_seconds, view_count.
-- Returning a string from a callback shows it as a status message.

yt.menu_item("Trending", "\239\131\167", function()
  yt.show_list("Trending", "https://www.youtube.com/feed/trending")
end)

yt.menu_item("Lofi Radio", "\239\128\129", function()
  local videos, err = yt.search(":live lofi hip hop radio", 15)
  if not videos then
    return "search failed: " .. err
  end
  yt.show_list("Lofi Radio", videos)
end)

-- Show only videos shorter than 10 minutes from the same channel.
yt.video_action("Short videos from this channel", "\239\128\151", function(video)
  if video.channel_url == "" then
    return "no channel for this video"
  end
  local videos = yt.ytdlp(video.channel_url .. "/videos", 60) or {}
  local short = {}
  for _, v in ipairs(videos) do
    if v.duration_seconds and v.duration_seconds < 600 then
      table.insert(short, v)
    end
  end
  yt.show_list(video.channel .. " · under 10 min", short)
end)

yt.video_action("Copy title", "\239\131\133", function(video)
  yt.shell("printf %s " .. string.format("%q", video.title) .. " | wl-copy")
  return "copied: " .. video.title
end)

-- Log everything you play to a file.
yt.on("play", function(video)
  local f = io.open(os.getenv("HOME") .. "/.cache/reelterm/played.log", "a")
  if f then
    f:write(os.date("%Y-%m-%d %H:%M") .. "  " .. video.title .. "  " .. video.url .. "\n")
    f:close()
  end
end)
