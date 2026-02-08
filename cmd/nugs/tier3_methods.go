package main

// Tier 3 methods have been moved to internal/model/progress_box.go.
// ProgressBoxState is a type alias for model.ProgressBoxState,
// so all methods (SetMessage, GetDisplayMessage, RequestRender,
// ResetForNewAlbum, SetPhase, ShouldRender, ShouldRenderLocked,
// etc.) are inherited automatically via the alias.
//
// getQualityName is re-exported via format.go.
