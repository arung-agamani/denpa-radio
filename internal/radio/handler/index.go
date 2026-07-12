package handler

import "github.com/gin-gonic/gin"

// HandlerDeps bundles all handler instances so that RegisterRoutes can wire
// them onto the gin engine without knowing about Server.
type HandlerDeps struct {
	TrackH    *TrackHandlers
	PlaylistH *PlaylistHandlers
	RadioH    *RadioHandlers
	LibraryH  *LibraryHandlers
	TimezoneH *TimezoneHandlers
	AuthH     *AuthHandlers
	SpaH      *SPAHandler
	ChannelH  *ChannelHandlers
}

// RegisterRoutes wires all routes onto the gin engine.
// authMiddleware is the JWT middleware (e.g. radio.AuthRequired(authInstance))
// passed in from the radio package to avoid an import cycle.
func RegisterRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc, deps HandlerDeps) {
	// --- Public non-API ---
	engine.GET("/health", deps.RadioH.Health)

	// --- Auth ---
	authGroup := engine.Group("/api/auth")
	{
		authGroup.POST("/login", deps.AuthH.Login)
		authGroup.GET("/verify", authMiddleware, deps.AuthH.VerifyToken)
	}

	// --- Public API ---
	api := engine.Group("/api")
	{
		api.GET("/status", deps.RadioH.Status)
		api.GET("/scheduler/status", deps.RadioH.SchedulerStatus)
		api.GET("/timezone", deps.TimezoneH.GetTimezone)
		api.GET("/master", deps.PlaylistH.Get)
		api.GET("/queue", deps.RadioH.GetQueue)
		api.GET("/timeslots", deps.PlaylistH.GetTimeSlots)

		// Literal sub-paths registered before :id to avoid routing conflicts.
		api.GET("/tracks/search", deps.TrackH.Search)
		api.GET("/tracks", deps.TrackH.List)
		api.GET("/tracks/:id", deps.TrackH.GetByID)
		api.GET("/tracks/:id/cover", deps.TrackH.Cover) // album art

		api.GET("/playlists", deps.PlaylistH.List)
		api.GET("/playlists/:id", deps.PlaylistH.GetByID)

		api.GET("/channels", deps.ChannelH.List)
		api.GET("/channels/:slug", deps.ChannelH.Get)
		api.GET("/channels/:slug/status", deps.ChannelH.GetStatus)
		api.GET("/channels/:slug/queue", deps.ChannelH.GetQueue)
	}

	// --- Protected API (JWT required) ---
	protected := engine.Group("/api")
	protected.Use(authMiddleware)
	{
		protected.POST("/channels", deps.ChannelH.Create)
		protected.PUT("/channels/:slug", deps.ChannelH.Update)
		protected.DELETE("/channels/:slug", deps.ChannelH.Delete)
		protected.PUT("/channels/:slug/master/:tag", deps.ChannelH.AssignPlaylistToTag)
		protected.DELETE("/channels/:slug/master/:tag/:playlistId", deps.ChannelH.RemovePlaylistFromTag)
		protected.PUT("/channels/:slug/timeslots", deps.ChannelH.SetTimeSlots)
		protected.POST("/channels/:slug/skip/next", deps.ChannelH.SkipNext)
		protected.POST("/channels/:slug/skip/prev", deps.ChannelH.SkipPrev)

		// Track management
		protected.GET("/tracks/orphaned", deps.TrackH.ListOrphaned)
		protected.PUT("/tracks/:id", deps.TrackH.Update)
		protected.DELETE("/tracks/:id", deps.TrackH.Delete)
		protected.POST("/tracks/scan", deps.LibraryH.Scan)
		protected.POST("/tracks/refresh-metadata", deps.LibraryH.RefreshMetadata)
		protected.POST("/tracks/upload", deps.TrackH.Upload)
		protected.POST("/tracks/:id/enrich", deps.TrackH.Enrich) // enrich single track

		// Library management
		protected.POST("/library/enrich", deps.LibraryH.EnrichAll) // batch enrich
		protected.POST("/library/batch-update", deps.LibraryH.BatchUpdate)
		protected.POST("/library/batch-cover", deps.LibraryH.BatchUpdateCover)

		// Playlist CRUD
		protected.POST("/playlists", deps.PlaylistH.Create)
		protected.PUT("/playlists/:id", deps.PlaylistH.Update)
		protected.DELETE("/playlists/:id", deps.PlaylistH.Delete)

		// Playlist track manipulation
		protected.POST("/playlists/:id/tracks", deps.PlaylistH.AddTrack)
		protected.DELETE("/playlists/:id/tracks/:trackId", deps.PlaylistH.RemoveTrack)
		protected.POST("/playlists/:id/tracks/move", deps.PlaylistH.MoveTrack)
		protected.POST("/playlists/:id/shuffle", deps.PlaylistH.Shuffle)

		// Playlist export / import
		protected.GET("/playlists/:id/export", deps.PlaylistH.Export)
		protected.POST("/playlists/import", deps.PlaylistH.Import)

		// Master playlist tag management
		protected.PUT("/master/:tag", deps.PlaylistH.AssignPlaylistToTag)
		protected.DELETE("/master/:tag/:playlistId", deps.PlaylistH.RemovePlaylistFromTag)

		// Time slot configuration
		protected.PUT("/timeslots", deps.PlaylistH.SetTimeSlots)

		// Reconcile & timezone
		protected.POST("/reconcile", deps.LibraryH.Reconcile)
		protected.PUT("/timezone", deps.TimezoneH.SetTimezone)

		// Skip controls
		protected.POST("/skip/next", deps.RadioH.SkipNext)
		protected.POST("/skip/prev", deps.RadioH.SkipPrev)
	}

	// --- SPA fallback (must be last) ---
	engine.NoRoute(deps.SpaH.Handle)
}
