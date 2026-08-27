#version 410 core
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec4 uTint;

out vec4 FragColor;

void main() {
    vec4 texColor = texture(uTexture, vTexCoord);
    if (texColor.a < 0.02) {
        discard;
    }
    FragColor = vec4(texColor.rgb * uTint.rgb, texColor.a * uTint.a);
}
