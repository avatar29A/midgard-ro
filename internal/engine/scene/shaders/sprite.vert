#version 410 core
layout (location = 0) in vec2 aPosition;
layout (location = 1) in vec2 aTexCoord;

uniform mat4 uViewProj;
uniform vec3 uWorldPos;
uniform vec2 uSpriteSize;
uniform vec3 uCamRight;  // Camera right vector for billboard
uniform vec3 uCamUp;     // Camera up vector for billboard

// How far toward the camera the whole sprite is depth-tested, in world units.
//
// Zero leaves every corner at its own depth, which is what a flat quad lying
// on the ground wants. Anything above zero tests the whole sprite at the
// depth of the point it stands on instead — see below.
uniform float uDepthLift;

out vec2 vTexCoord;

void main() {
    // Camera-facing billboard: sprite always faces the camera
    // This creates the 3D illusion when combined with directional sprite frames
    vec3 pos = uWorldPos;
    pos += uCamRight * aPosition.x * uSpriteSize.x;
    pos += uCamUp * aPosition.y * uSpriteSize.y;

    vTexCoord = aTexCoord;

    vec4 corner = uViewProj * vec4(pos, 1.0);

    // A unit is at one place — where it stands — and whether something hides
    // it is a question about that place, not about each pixel of the quad
    // drawn for it. Tested per corner, a sprite standing on rising ground has
    // its lower half fail against the very slope it is standing on, and it
    // sinks into the hill.
    //
    // So the depth comes from the foot of the sprite, lifted a little toward
    // the camera because otherwise it ties with the ground it rests on and
    // loses. Terrain in front of that point still hides the unit, which is
    // what standing behind a hill should do.
    if (uDepthLift > 0.0) {
        vec3 toCamera = normalize(cross(uCamUp, uCamRight));
        vec4 anchor = uViewProj * vec4(uWorldPos + toCamera * uDepthLift, 1.0);

        corner.z = anchor.z / anchor.w * corner.w;
    }

    gl_Position = corner;
}
