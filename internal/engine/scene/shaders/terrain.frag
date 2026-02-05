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

// Point lights (RSW light sources)
const int MAX_POINT_LIGHTS = 32;
uniform vec3 uPointLightPositions[MAX_POINT_LIGHTS];
uniform vec3 uPointLightColors[MAX_POINT_LIGHTS];
uniform float uPointLightRanges[MAX_POINT_LIGHTS];
uniform float uPointLightIntensities[MAX_POINT_LIGHTS];
uniform int uPointLightCount;
uniform bool uPointLightsEnabled;

out vec4 FragColor;

// Calculate point light contribution using quadratic attenuation
vec3 calculatePointLights(vec3 worldPos, vec3 normal) {
    if (!uPointLightsEnabled || uPointLightCount <= 0) {
        return vec3(0.0);
    }

    vec3 totalLight = vec3(0.0);

    for (int i = 0; i < uPointLightCount && i < MAX_POINT_LIGHTS; i++) {
        vec3 lightPos = uPointLightPositions[i];
        vec3 lightColor = uPointLightColors[i];
        float lightRange = uPointLightRanges[i];
        float lightIntensity = uPointLightIntensities[i];

        // Direction from fragment to light
        vec3 lightDir = lightPos - worldPos;
        float distance = length(lightDir);

        // Skip if outside light range
        if (distance > lightRange) {
            continue;
        }

        lightDir = normalize(lightDir);

        // Quadratic attenuation with smooth falloff at range boundary
        float attenuation = 1.0 - (distance / lightRange);
        attenuation = attenuation * attenuation;  // Quadratic falloff

        // Simple diffuse lighting (half-lambert for softer shadows)
        float NdotL = dot(normal, lightDir) * 0.5 + 0.5;

        // Accumulate light contribution
        totalLight += lightColor * lightIntensity * NdotL * attenuation;
    }

    return totalLight;
}

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

    // Lightmap: RGB = color tint (Korangar style: baked shadows are in vertex colors, not lightmap alpha)
    vec4 lightmap = texture(uLightmap, vLightmapUV);
    vec3 colorTint = lightmap.rgb;   // Color tint only

    // Real-time shadow from shadow map (softened to 50% intensity)
    float realtimeShadow = calculateShadow();
    realtimeShadow = mix(1.0, realtimeShadow, 0.5);  // Softer shadows

    // Baked shadows are now in vertex colors (vColor), not lightmap alpha
    // This gives smooth interpolation across tile boundaries (Korangar approach)
    float combinedShadow = realtimeShadow;

    // Directional light component (sun)
    // Use half-lambert for softer lighting that reduces visible triangle seams
    vec3 normal = normalize(vNormal);
    vec3 lightDir = normalize(uLightDir);
    float NdotL = dot(normal, lightDir) * 0.5 + 0.5;  // Half-lambert wrap
    NdotL = NdotL * NdotL;  // Square for slightly sharper falloff
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

    // Add point light contributions (RSW light sources)
    vec3 pointLightContrib = calculatePointLights(vWorldPos, normal);
    lighting += pointLightContrib;

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

    // Apply warm color tint (Korangar-style golden hour atmosphere)
    vec3 warmTint = vec3(1.08, 1.02, 0.92);  // Stronger warm/golden shift
    finalColor = finalColor * warmTint;

    // Apply fog (roBrowser formula using smoothstep)
    if (uFogUse) {
        float depth = gl_FragCoord.z / gl_FragCoord.w;
        float fogFactor = smoothstep(uFogNear, uFogFar, depth);
        finalColor = mix(finalColor, uFogColor, fogFactor);
    }

    FragColor = vec4(finalColor, texColor.a * vColor.a);
}
