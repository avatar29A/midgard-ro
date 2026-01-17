#version 410 core
layout (location = 0) in vec3 aPosition;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec2 aTexCoord;
layout (location = 3) in vec2 aLightmapUV;
layout (location = 4) in vec4 aColor;

uniform mat4 uViewProj;

out vec3 vNormal;
out vec2 vTexCoord;
out vec2 vLightmapUV;
out vec4 vColor;

void main() {
    vNormal = aNormal;
    vTexCoord = aTexCoord;
    vLightmapUV = aLightmapUV;
    vColor = aColor;
    gl_Position = uViewProj * vec4(aPosition, 1.0);
}
