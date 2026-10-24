package app

func resumeOptionsForJob(
	job Job,
	resumes []Resume,
	families []RoleFamily,
) []documentRow {
	familyNames := make(map[int64]string, len(families))

	for _, family := range families {
		familyNames[family.ID] = family.Name
	}

	options := make([]documentRow, 0, len(resumes))

	for _, resume := range resumes {
		if resume.IsMaster ||
			(resume.ArchivedAt != "" &&
				resume.ID != job.SelectedResumeID) {
			continue
		}

		options = append(options, documentRow{
			Resume:     resume,
			FamilyName: familyNames[resume.RoleFamilyID],
		})
	}

	return options
}
