#version 410 core
in vec3 vNormal;
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec3 uLightDir;
uniform vec3 uAmbient;
uniform vec3 uDiffuse;

// Fog uniforms (roBrowser style)
uniform bool uFogUse;
uniform float uFogNear;
uniform float uFogFar;
uniform vec3 uFogColor;

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);

    // Discard transparent pixels (alpha set to 0 for magenta color key during texture load)
    if (texColor.a < 0.5) {
        discard;
    }

    // Simple lighting with shadow lift (roBrowser uses min 0.5 for models)
    float NdotL = max(dot(normalize(vNormal), normalize(uLightDir)), 0.5);
    vec3 lighting = uAmbient + uDiffuse * NdotL;

    vec3 color = texColor.rgb * lighting;

    // Apply fog (roBrowser formula using smoothstep)
    if (uFogUse) {
        float depth = gl_FragCoord.z / gl_FragCoord.w;
        float fogFactor = smoothstep(uFogNear, uFogFar, depth);
        color = mix(color, uFogColor, fogFactor);
    }

    FragColor = vec4(color, texColor.a);
}
