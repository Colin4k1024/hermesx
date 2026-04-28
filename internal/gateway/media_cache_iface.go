package gateway

// MediaCacher is the interface for media file caching.
// Implemented by MediaCache (local filesystem) and future S3-backed implementations.
type MediaCacher interface {
	CacheImageFromURL(url string) (string, error)
	CacheImageFromBytes(data []byte, ext string) (string, error)
	CacheAudioFromURL(url string) (string, error)
	CacheDocumentFromBytes(data []byte, filename string) (string, error)
	CleanupCache(maxAgeHours int) int
	CacheDir() string
}
