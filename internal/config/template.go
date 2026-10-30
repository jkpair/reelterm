package config

const confTemplate = `#
#    yt-x config
#

# plugins loaded on every start (comma-separated file names in the plugins folder, .lua optional)
AUTO_LOADED_EXTENSIONS: {{join .AutoLoadedPlugins}}

# whether to show colors when printing ouput
PRETTY_PRINT: {{.PrettyPrint}}

# color theme (run yt-x --list-themes)
# themes are loaded from ~/.config/yt-x/themes/ or the built-in themes
THEME: {{.Theme}}

# your preferred editor for editing your config
EDITOR: {{.Editor}}

# the quality of the video when streaming with a player other than mpv
VIDEO_QUALITY: {{.VideoQuality}}

# whether to show previews [true/false]
ENABLE_PREVIEW: {{.EnablePreview}}

# how to draw thumbnails [auto/kitty/sixel/iterm/halfblock/chafa]
# auto picks the best protocol your terminal supports (halfblock on Windows;
# Windows Terminal 1.22+ users can try sixel)
IMAGE_RENDERER: {{.ImageRenderer}}

# resume videos where you stopped watching (mpv) [true/false]
RESUME_PLAYBACK: {{.ResumePlayback}}

# whether to keep the player running after yt-x exits
DISOWN_STREAMING_PROCESS: {{.DisownStreaming}}

# whether to update the recent list kept locally [true/false]
UPDATE_RECENT: {{.UpdateRecent}}

# whether to keep a search history [true/false]
SEARCH_HISTORY: {{.SearchHistory}}

# the number of recent videos to keep
NO_OF_RECENT: {{.NoOfRecent}}

# the player to use for streaming [mpv/vlc]
PLAYER: {{.Player}}

# the browser to use to extract cookies from
# this is used by yt-dlp to access content that would require login
PREFERRED_BROWSER: {{.PreferredBrowser}}

# fallback cookie file path (used if browser cookie extraction fails)
COOKIE_FILE: {{.CookieFile}}

# the number of results to get from yt-dlp
NO_OF_SEARCH_RESULTS: {{.NoOfSearchResults}}

# the number of videos per page on the feed
FEED_PAGE_SIZE: {{.FeedPageSize}}

# seconds before the cached feed is refreshed
FEED_CACHE_TTL: {{.FeedCacheTTL}}

# the duration notifications stay on the screen
NOTIFICATION_DURATION: {{.NotificationDuration}}

# where your downloads will be stored
DOWNLOAD_DIRECTORY: {{.DownloadDirectory}}

# whether to enable the welcome screeen which runs once a month [true/false]
WELCOME_SCREEN: {{.WelcomeScreen}}

# ===== yt-dlp Enhanced Options =====

# skip sponsored segments using SponsorBlock [true/false]
SPONSORBLOCK: {{.SponsorBlock}}

# SponsorBlock categories to skip (comma-separated)
# options: sponsor,intro,outro,selfpromo,preview,filler,interaction,music_offtopic,poi_highlight,chapter,all
SPONSORBLOCK_CATS: {{.SponsorBlockCats}}

# mark videos as watched on YouTube after viewing [true/false]
MARK_WATCHED: {{.MarkWatched}}

# download live streams from the start [true/false]
LIVE_FROM_START: {{.LiveFromStart}}

# number of concurrent fragment downloads (1-16)
CONCURRENT_FRAGMENTS: {{.ConcurrentFragments}}

# number of retries for failed downloads
DOWNLOAD_RETRIES: {{.DownloadRetries}}

# embed thumbnail in downloaded files [true/false]
EMBED_THUMBNAIL: {{.EmbedThumbnail}}

# embed subtitles in downloaded files [true/false]
EMBED_SUBS: {{.EmbedSubs}}

# embed chapters/metadata in downloaded files [true/false]
EMBED_CHAPTERS: {{.EmbedChapters}}

# preferred video codec [av1/vp9/h264/any]
PREFERRED_CODEC: {{.PreferredCodec}}

# prefer HDR content when available [true/false]
PREFER_HDR: {{.PreferHDR}}

# subtitle languages to download (comma-separated, e.g., en,es,auto)
SUBTITLE_LANGS: {{.SubtitleLangs}}
`
