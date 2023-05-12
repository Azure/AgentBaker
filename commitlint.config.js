// https://github.com/conventional-changelog/commitlint/tree/master/%40commitlint/config-conventional
module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [
			2,
			'always',
			[
        // can add to this list for customization
				'build',
				'bump',
				'chore',
				'ci',
				'docs',
				'feat',
				'fix',
				'perf',
				'refactor',
				'revert',
				'style',
				'test',
			],
		],
		'footer-max-line-length': [1, 'always', 100], // allow long link refs
  },
	parserPreset: { parserOpts: { noteKeywords: ['\\[.+\\]:'] } }, // allow long link refs
}
