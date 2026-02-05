#version 410 core
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec4 uTint;

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);

    // Discard transparent pixels
    if (texColor.a < 0.1) {
        discard;
    }

    FragColor = texColor * uTint;
}
