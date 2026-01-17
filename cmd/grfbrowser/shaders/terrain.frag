#version 410 core
in vec3 vNormal;
in vec2 vTexCoord;
in vec2 vLightmapUV;
in vec4 vColor;
in vec3 vWorldPos;
in vec4 vLightSpacePos;

uniform sampler2D uTexture;
uniform sampler2D uLightmap;
uniform sampler2DShadow uShadowMap;  // Shadow map with comparison mode
uniform vec3 uLightDir;
uniform vec3 uAmbient;
uniform vec3 uDiffuse;
uniform float uBrightness;
uniform float uLightOpacity;
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
    float bias = 0.002;
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

    // Discard transparent pixels (magenta key areas)
    if (texColor.a < 0.5) {
        discard;
    }

    // Lightmap: RGB = color tint, A = shadow intensity (0=shadow, 1=lit)
    vec4 lightmap = texture(uLightmap, vLightmapUV);
    float bakedShadow = lightmap.a;  // 0.0 = full shadow, 1.0 = fully lit (baked)
    vec3 colorTint = lightmap.rgb;   // Color tint (0-255 normalized by GPU)

    // Real-time shadow from shadow map
    float realtimeShadow = calculateShadow();

    // Combine baked and real-time shadows (multiply for darkest)
    float combinedShadow = bakedShadow * realtimeShadow;

    // Directional light component (sun)
    vec3 normal = normalize(vNormal);
    vec3 lightDir = normalize(uLightDir);
    float NdotL = max(dot(normal, lightDir), 0.0);
    vec3 directional = uDiffuse * NdotL;

    // Lighting formula:
    // Ambient provides base illumination (not fully shadowed)
    // Directional light (sun) is affected by combined shadows
    // Opacity controls shadow visibility (higher = darker shadows)
    vec3 ambient = uAmbient;

    // Shadow affects directional light, ambient provides minimum illumination
    // Mix ambient shadow based on opacity (0 = no shadow effect, 1 = full shadow)
    float ambientShadow = mix(1.0, combinedShadow, uLightOpacity);
    vec3 lighting = ambient * ambientShadow + directional * combinedShadow;

    // Clamp lighting to [0, 1] range (prevents overbright)
    lighting = clamp(lighting, vec3(0.0), vec3(1.0));

    // Ensure vertex color doesn't cause black (default to white if black)
    vec3 vertColor = vColor.rgb;
    if (vertColor.r + vertColor.g + vertColor.b < 0.1) {
        vertColor = vec3(1.0);
    }

    // Final color: (texture * lighting * vertColor * brightness) + colorTint
    // roBrowser formula: texture * LightColor + ColorMap
    vec3 finalColor = texColor.rgb * lighting * vertColor * uBrightness + colorTint;

    // Apply fog (roBrowser formula using smoothstep)
    if (uFogUse) {
        float depth = gl_FragCoord.z / gl_FragCoord.w;
        float fogFactor = smoothstep(uFogNear, uFogFar, depth);
        finalColor = mix(finalColor, uFogColor, fogFactor);
    }

    FragColor = vec4(finalColor, texColor.a * vColor.a);
}
