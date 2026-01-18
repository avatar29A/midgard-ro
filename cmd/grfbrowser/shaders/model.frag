#version 410 core
in vec3 vNormal;
in vec2 vTexCoord;
in vec3 vWorldPos;
in vec4 vLightSpacePos;

uniform sampler2D uTexture;
uniform sampler2DShadow uShadowMap;  // Shadow map with comparison mode
uniform vec3 uLightDir;
uniform vec3 uAmbient;
uniform vec3 uDiffuse;
uniform bool uShadowsEnabled;        // Toggle for real-time shadows

// Fog uniforms (roBrowser style)
uniform bool uFogUse;
uniform float uFogNear;
uniform float uFogFar;
uniform vec3 uFogColor;

out vec4 FragColor;

// PCF shadow sampling with 3x3 kernel for soft edges
float calculateShadow() {
    if (!uShadowsEnabled) {
        return 1.0;  // Fully lit if shadows disabled
    }

    // Perspective divide
    vec3 projCoords = vLightSpacePos.xyz / vLightSpacePos.w;
    // Transform to [0,1] range
    projCoords = projCoords * 0.5 + 0.5;

    // If outside shadow map bounds, no shadow
    if (projCoords.z > 1.0 || projCoords.x < 0.0 || projCoords.x > 1.0 ||
        projCoords.y < 0.0 || projCoords.y > 1.0) {
        return 1.0;
    }

    // PCF 3x3 sampling for soft shadow edges
    float shadow = 0.0;
    vec2 texelSize = 1.0 / textureSize(uShadowMap, 0);

    // Apply small bias to reduce shadow acne
    float bias = 0.003;
    float currentDepth = projCoords.z - bias;

    for (int x = -1; x <= 1; x++) {
        for (int y = -1; y <= 1; y++) {
            vec2 offset = vec2(float(x), float(y)) * texelSize;
            shadow += texture(uShadowMap, vec3(projCoords.xy + offset, currentDepth));
        }
    }
    shadow /= 9.0;

    return shadow;
}

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);

    // Discard transparent pixels (alpha set to 0 for magenta color key during texture load)
    if (texColor.a < 0.5) {
        discard;
    }

    // Real-time shadow from shadow map (softened to 50% intensity)
    float shadowFactor = calculateShadow();
    shadowFactor = mix(1.0, shadowFactor, 0.5);  // Softer shadows

    // Lighting with shadow (roBrowser uses min 0.5 for models)
    vec3 normal = normalize(vNormal);
    vec3 lightDir = normalize(uLightDir);
    float NdotL = max(dot(normal, lightDir), 0.0);

    // Apply shadow to directional component, keep ambient
    // Mix between shadowed (ambient only) and lit (ambient + diffuse)
    vec3 ambient = uAmbient;
    vec3 diffuse = uDiffuse * NdotL * shadowFactor;

    // roBrowser style: models have a minimum brightness of 0.5
    vec3 lighting = ambient + diffuse;
    lighting = max(lighting, vec3(0.5));

    vec3 color = texColor.rgb * lighting;

    // Apply warm color tint (Korangar-style golden hour atmosphere)
    vec3 warmTint = vec3(1.08, 1.02, 0.92);  // Stronger warm/golden shift
    color = color * warmTint;

    // Apply fog (roBrowser formula using smoothstep)
    if (uFogUse) {
        float depth = gl_FragCoord.z / gl_FragCoord.w;
        float fogFactor = smoothstep(uFogNear, uFogFar, depth);
        color = mix(color, uFogColor, fogFactor);
    }

    FragColor = vec4(color, texColor.a);
}
