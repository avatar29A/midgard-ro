#version 410 core
layout (location = 0) in vec2 aPosition;
layout (location = 1) in vec2 aTexCoord;

uniform mat4 uViewProj;
uniform vec3 uWorldPos;
uniform vec2 uSpriteSize;
uniform vec3 uCamRight;  // Camera right vector for billboard
uniform vec3 uCamUp;     // Camera up vector for billboard

// How far toward the camera the sprite is moved before its depth is measured,
// in world units. Zero leaves every corner at its own depth, which is what a
// flat quad lying on the ground wants.
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

    // Depth: one value for the whole sprite, taken where it stands and moved
    // toward the camera before it is measured.
    //
    // One value because a unit is somewhere. It is in front of the barrel or
    // behind it, not both. Measured per corner instead, the quad's own lean
    // decides: it leans back to face the camera and leans further the further
    // it is from the middle of the screen, so a sprite's head comes out
    // deeper than its feet by however far off centre it happens to be, and a
    // barrel behind it is drawn over the head while the body stays in front.
    // Reading up the sprite's world column is no better — a head really is
    // further from a camera looking down than the feet are, so measured there
    // a sprite loses to whatever the camera can see over it, which is
    // everything.
    //
    // Where it stands is the middle of the bottom edge rather than the middle
    // of the quad: the sprite reaches the ground there, and taken any higher
    // the ground in front wins and cuts the sprite off in a straight line.
    //
    // And the shift is what stops it losing to the ground it rests on, which
    // is at exactly its own depth. In world units rather than in depth, so
    // the projection scales it and one number holds both close up, where a
    // hair of depth is a long way, and far off, where it is nothing.
    if (uDepthLift > 0.0) {
        vec3 toCamera = normalize(cross(uCamUp, uCamRight));
        vec3 foot = uWorldPos + uCamRight * (0.5 * uSpriteSize.x);

        vec4 shifted = uViewProj * vec4(foot + toCamera * uDepthLift, 1.0);

        corner.z = shifted.z / shifted.w * corner.w;
    }

    gl_Position = corner;
}
