#version 410 core

// Shadow pass fragment shader
// Depth is written automatically by the GPU
// This shader only needs to exist for the program to link

void main() {
    // Depth is written automatically to the depth buffer
    // No color output needed (we disabled color writes for shadow FBO)
}
