void main() {
#ifdef LIGHT
	gl_FragColor = vec4(0.0, 0.0, 0.0, 1.0);
#else
	vec2 q = vec2(vtex);
	vec3 col = texture2D(t0, q).rgb;
	float band = exp(-pow((q.y - u[5] - 0.06) * 9.0, 2.0));
	float drift = 0.5 + 0.25 * sin(q.x * 5.0 + u[2] * 6.2832) + 0.25 * sin(q.x * 11.0 - u[3] * 6.2832 + q.y * 7.0);
	col += vec3(0.03, 0.045, 0.045) * band * drift;
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
