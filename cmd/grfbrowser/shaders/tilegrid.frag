#version 410 core
in vec4 vColor;

out vec4 FragColor;

void main() {
    // Use vertex color with guaranteed visibility
    FragColor = vec4(vColor.rgb, vColor.a);
}
