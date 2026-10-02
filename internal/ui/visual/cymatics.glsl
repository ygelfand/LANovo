void main() {
#ifdef LIGHT
	gl_FragColor = vec4(0.0, 0.0, 0.0, 1.0);
#else
	gl_FragColor = vec4(texture2D(t0, vtex).rgb, 1.0);
#endif
}
