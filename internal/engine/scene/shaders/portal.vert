#version 410 core
// The warp portal: a unit tube (xz on the unit circle, y 0 at the base and 1
// at the top) placed, sized, stretched and spun by uniforms, so one mesh
// serves every portal on the map — and, with no height, the disc under it.
layout (location = 0) in vec3 aPosition;
layout (location = 1) in vec2 aTexCoord;

uniform mat4 uViewProj;
uniform vec3 uPosition;
uniform float uBottomSize;
uniform float uTopSize;
uniform float uHeight;
uniform float uSpin;      // radians about the tube's axis

out vec2 vTexCoord;

void main() {
    float size = mix(uBottomSize, uTopSize, aPosition.y);
    float c = cos(uSpin);
    float s = sin(uSpin);
    vec2 xz = vec2(aPosition.x * c - aPosition.z * s, aPosition.x * s + aPosition.z * c) * size;
    vec3 pos = uPosition + vec3(xz.x, aPosition.y * uHeight, xz.y);

    vTexCoord = aTexCoord;
    gl_Position = uViewProj * vec4(pos, 1.0);
}
