#version 410 core
in vec3 vWorldPos;

uniform vec4 uWaterColor;
uniform float uTime;
uniform float uScrollSpeed;
uniform sampler2D uWaterTex;
uniform int uUseTexture;

out vec4 FragColor;

// Hash function for pseudo-random noise (fallback)
float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    float a = hash(i);
    float b = hash(i + vec2(1.0, 0.0));
    float c = hash(i + vec2(0.0, 1.0));
    float d = hash(i + vec2(1.0, 1.0));
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

float fbm(vec2 p, float time) {
    float value = 0.0;
    float amplitude = 0.5;
    vec2 shift = vec2(time * 0.3, time * 0.2);
    for (int i = 0; i < 4; i++) {
        value += amplitude * noise(p + shift);
        p = p * 2.0 + vec2(1.7, 9.2);
        shift *= 1.1;
        amplitude *= 0.5;
    }
    return value;
}

void main() {
    // Scale world position for texture coordinates - tile the texture
    // RO tiles water texture approximately every 50-100 world units
    vec2 uv = vWorldPos.xz * 0.02; // Tiling scale

    if (uUseTexture == 1) {
        // Use loaded water texture - frame animation creates shimmering effect
        // No UV scrolling - just tile the texture
        vec2 tileUV = vWorldPos.xz * 0.004;
        vec4 texColor = texture(uWaterTex, tileUV);
        FragColor = vec4(texColor.rgb, 1.0);
    } else {
        // Fallback to procedural water
        vec2 procUV = vWorldPos.xz * 0.05;
        float pattern1 = fbm(procUV, uTime);
        float pattern2 = fbm(procUV * 1.5 + vec2(5.0), uTime * 0.8);
        float pattern = mix(pattern1, pattern2, 0.5);

        vec3 deepColor = vec3(0.12, 0.30, 0.45);
        vec3 midColor = vec3(0.20, 0.45, 0.55);
        vec3 lightColor = vec3(0.35, 0.60, 0.70);

        vec3 waterColor;
        if (pattern < 0.4) {
            waterColor = mix(deepColor, midColor, pattern / 0.4);
        } else {
            waterColor = mix(midColor, lightColor, (pattern - 0.4) / 0.6);
        }
        float caustic = pow(pattern, 2.5) * 0.4;
        waterColor += vec3(caustic * 0.5, caustic * 0.7, caustic);

        FragColor = vec4(waterColor, uWaterColor.a);
    }
}
