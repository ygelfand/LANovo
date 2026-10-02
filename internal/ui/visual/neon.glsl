void main() {
	vec4 m = texture2D(t1, vtex);
	float letter = floor(m.a * 255.0 + 0.5);
	float on = abs(letter - u[4]) < 0.5 ? u[5] : 1.0;
	float I = u[0] * on;
	vec3 c = vec3(u[1], u[2], u[3]);
	vec3 tube = mix(c, vec3(1.0), 0.65) * m.r * I;
	vec3 glowc = c * m.g * I * 0.85;
#ifdef LIGHT
	gl_FragColor = vec4(tube + glowc, 1.0);
#else
	vec3 wall = texture2D(t0, vtex).rgb;
	vec3 col = wall * (0.45 + c * m.b * u[0] * 3.2) + glowc + tube;
	gl_FragColor = vec4(min(col + glow() * c, 1.0), 1.0);
#endif
}
