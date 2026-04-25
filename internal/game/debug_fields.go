package game

import (
	"time"

	"github.com/Faultbox/midgard-ro/internal/game/states"
	"github.com/Faultbox/midgard-ro/internal/game/ui"
	"github.com/Faultbox/midgard-ro/internal/network"
)

// populateDebugFields fills the diagnostic fields of an InGameUIState from
// live state — camera, scene framebuffer, GL error, terrain Y at the player
// position, and network telemetry. The overlay only displays them when
// state.ShowDebugInfo is true (toggled by F3 in game.go), but populating
// them every frame keeps the readout live the moment the user presses F3.
func populateDebugFields(out *ui.InGameUIState, state *states.InGameState, client *network.Client) {
	if state == nil {
		return
	}

	if player := state.GetPlayer(); player != nil {
		out.PlayerHasDest = player.HasDestination
		out.PlayerDestX = player.DestX
		out.PlayerDestZ = player.DestZ
		out.PlayerIsMoving = player.IsMoving

		if sc := state.GetScene(); sc != nil {
			w, h := sc.FramebufferSize()
			out.SceneFBWidth = w
			out.SceneFBHeight = h
			out.SceneTexID = sc.ColorTexture()
			out.TerrainY = sc.GetTerrainHeight(player.WorldX, player.WorldZ)
		}
	}

	if cam := state.GetCamera(); cam != nil {
		out.CamX = cam.PosX
		out.CamY = cam.PosY
		out.CamZ = cam.PosZ
		out.CamDistance = cam.Distance
		out.CamYaw = cam.Yaw
		out.CamPitch = cam.Pitch
	}

	// gl.GetError consumed in the UI layer (already imports gl). Sampled
	// once per frame is enough — overlays that read it from there will see
	// the most recent error flag.

	if client != nil {
		st := client.Stats()
		out.PacketsSent = st.PacketsSent
		out.PacketsReceived = st.PacketsRecvd
		out.BytesSent = st.BytesSent
		out.BytesReceived = st.BytesRecvd
		out.LastSentID = st.LastSentID
		out.LastSentLen = st.LastSentLen
		out.LastRecvID = st.LastRecvID
		out.LastRecvLen = st.LastRecvLen
		now := time.Now()
		if !st.LastSentAt.IsZero() {
			out.LastSentAgoMs = now.Sub(st.LastSentAt).Milliseconds()
		}
		if !st.LastRecvAt.IsZero() {
			out.LastRecvAgoMs = now.Sub(st.LastRecvAt).Milliseconds()
		}
	}
}
