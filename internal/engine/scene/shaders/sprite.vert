#version 410 core
layout (location = 0) in vec2 aPosition;
layout (location = 1) in vec2 aTexCoord;

uniform mat4 uViewProj;
uniform vec3 uWorldPos;
uniform vec2 uSpriteSize;
uniform vec3 uCamRight;  // Camera right vector for billboard
uniform vec3 uCamUp;     // Camera up vector for billboard

out vec2 vTexCoord;

void main() {
    // Camera-facing billboard: sprite always faces the camera
    // This creates the 3D illusion when combined with directional sprite frames
    vec3 pos = uWorldPos;
    pos += uCamRight * aPosition.x * uSpriteSize.x;
    pos += uCamUp * aPosition.y * uSpriteSize.y;

    vTexCoord = aTexCoord;
    gl_Position = uViewProj * vec4(pos, 1.0);
}
