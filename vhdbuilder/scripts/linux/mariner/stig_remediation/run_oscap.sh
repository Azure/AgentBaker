echo '    >>>> running oscap with input /home/mingheren/Desktop/git/stig/stig_scripts/ssg-mariner-ds.xml'
oscap xccdf eval --profile xccdf_org.ssgproject.content_profile_stig-rhel8 --stig-viewer stig-rhel8_results.xml --report stig-rhel8_report.html ssg-mariner-ds.xml 2>&1 | tee oscap.log
echo '    >>>> oscap log: /home/mingheren/Desktop/git/stig/stig_scripts/oscap.log'
echo '    >>>> oscap results: /home/mingheren/Desktop/git/stig/stig_scripts/stig-rhel8_results.xml'
echo '    >>>> oscap report: /home/mingheren/Desktop/git/stig/stig_scripts/stig-rhel8_report.html'
echo "    >>>> consider running 'chown mariner_user:mariner_user *' before SCP'ing files over"
