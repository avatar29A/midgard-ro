package camera

import "testing"

func TestIndoorLimitsForbidOrbiting(t *testing.T) {
	c := NewThirdPersonCamera()
	c.HandleYaw(100)
	if c.Yaw == 0 {
		t.Fatal("an unrestricted camera should have turned")
	}

	c.SetYaw(0)
	c.SetLimits(Limits{YawLocked: true})
	c.HandleYaw(100)
	if c.Yaw != 0 {
		t.Fatalf("a locked camera turned to %v", c.Yaw)
	}

	c.SetLimits(Limits{})
	c.HandleYaw(100)
	if c.Yaw == 0 {
		t.Fatal("lifting the limits should let it turn again")
	}
}

func TestArcLimitsClampTheYaw(t *testing.T) {
	c := NewThirdPersonCamera()
	c.SetLimits(Limits{Arc: true, YawMin: -0.5, YawMax: 0.5})

	c.HandleYaw(-100000)
	if c.Yaw != 0.5 {
		t.Fatalf("yaw %v, want the arc's upper bound 0.5", c.Yaw)
	}
	c.HandleYaw(100000)
	if c.Yaw != -0.5 {
		t.Fatalf("yaw %v, want the arc's lower bound -0.5", c.Yaw)
	}

	// Applying an arc to a camera already outside it pulls it in.
	c.SetLimits(Limits{})
	c.SetYaw(3)
	c.SetLimits(Limits{Arc: true, YawMin: -1, YawMax: 1})
	if c.Yaw != 1 {
		t.Fatalf("yaw %v after applying an arc, want 1", c.Yaw)
	}
}
