package service

import (
	"hpe-recipe/internal/integration/websocket"
	"hpe-recipe/internal/model"
)

// EventService provides broadcast operations for real-time WebSocket events.
type EventService interface {
	BroadcastStatusChange(version, status, cluster string)
	BroadcastVersionCreated(version string)
	BroadcastReleaseDeleted(version string)
	BroadcastRecipeAdded(helmVersion, cluster string, recipe *model.Recipe)
	BroadcastRecipeUpdated(helmVersion, cluster string, recipe *model.Recipe)
	BroadcastRecipeDeleted(helmVersion, cluster, recipeVersion string)
	BroadcastReleaseCreated(release *model.HelmRelease)
	BroadcastReleaseUpdated(release *model.HelmRelease)
}

type eventService struct {
	hub *websocket.Hub
}

// NewEventService initializes an EventService.
func NewEventService(hub *websocket.Hub) EventService {
	return &eventService{
		hub: hub,
	}
}

func (e *eventService) BroadcastStatusChange(version, status, cluster string) {
	e.hub.Broadcast("status_changed", map[string]string{
		"version": version,
		"status":  status,
		"cluster": cluster,
	})
}

func (e *eventService) BroadcastVersionCreated(version string) {
	e.hub.Broadcast("version_created", map[string]string{
		"version": version,
	})
}

func (e *eventService) BroadcastReleaseDeleted(version string) {
	e.hub.Broadcast("release_deleted", map[string]string{
		"version": version,
	})
}

func (e *eventService) BroadcastRecipeAdded(helmVersion, cluster string, recipe *model.Recipe) {
	e.hub.Broadcast("recipe_added", map[string]interface{}{
		"helmVersion": helmVersion,
		"cluster":     cluster,
		"recipe":      recipe,
	})
}

func (e *eventService) BroadcastRecipeUpdated(helmVersion, cluster string, recipe *model.Recipe) {
	e.hub.Broadcast("recipe_updated", map[string]interface{}{
		"helmVersion": helmVersion,
		"cluster":     cluster,
		"recipe":      recipe,
	})
}

func (e *eventService) BroadcastRecipeDeleted(helmVersion, cluster, recipeVersion string) {
	e.hub.Broadcast("recipe_deleted", map[string]string{
		"helmVersion":   helmVersion,
		"cluster":       cluster,
		"recipeVersion": recipeVersion,
	})
}

func (e *eventService) BroadcastReleaseCreated(release *model.HelmRelease) {
	e.hub.Broadcast("release_created", release)
}

func (e *eventService) BroadcastReleaseUpdated(release *model.HelmRelease) {
	e.hub.Broadcast("release_updated", release)
}
