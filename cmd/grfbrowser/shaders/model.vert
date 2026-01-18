#version 410 core
layout (location = 0) in vec3 aPosition;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec2 aTexCoord;

uniform mat4 uMVP;
uniform mat4 uModel;          // Model matrix for world position
uniform mat4 uLightViewProj;  // For shadow mapping

out vec3 vNormal;
out vec2 vTexCoord;
out vec3 vWorldPos;
out vec4 vLightSpacePos;

void main() {
    vec4 worldPos = uModel * vec4(aPosition, 1.0);
    vWorldPos = worldPos.xyz;
    vNormal = mat3(uModel) * aNormal;  // Transform normal to world space
    vTexCoord = aTexCoord;
    vLightSpacePos = uLightViewProj * worldPos;
    gl_Position = uMVP * vec4(aPosition, 1.0);
}
