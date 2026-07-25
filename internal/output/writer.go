package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainoffallingstar/seq2mat/pkg/dataframe"
)

const (
	countMatrixFilename      = "matrix_count.txt"
	normalizedMatrixFilename = "matrix_norm.txt"
)

type matrixFileWriter func(dataFrame *dataframe.DataFrame, filePath string) error
type fileRenameFunction func(oldPath, newPath string) error

type matrixSpecification struct {
	dataFrame *dataframe.DataFrame
	filename  string
}

type matrixPublication struct {
	targetPath    string
	temporaryPath string
	backupPath    string
	hadOriginal   bool
	published     bool
}

// WriteMatrix writes one DataFrame through a same-directory temporary file.
func WriteMatrix(dataFrame *dataframe.DataFrame, outputDirectory, filename string) error {
	return writeMatrixSet(
		[]matrixSpecification{{dataFrame: dataFrame, filename: filename}},
		outputDirectory,
		writeDataFrame,
		os.Rename,
	)
}

// WriteMatrices transactionally publishes the raw and normalized matrices.
func WriteMatrices(countDataFrame, normalizedDataFrame *dataframe.DataFrame, outputDirectory string) error {
	return writeMatrixSet(
		[]matrixSpecification{
			{dataFrame: countDataFrame, filename: countMatrixFilename},
			{dataFrame: normalizedDataFrame, filename: normalizedMatrixFilename},
		},
		outputDirectory,
		writeDataFrame,
		os.Rename,
	)
}

func writeDataFrame(dataFrame *dataframe.DataFrame, filePath string) error {
	return dataFrame.WriteTSV(filePath)
}

func writeMatrixSet(
	specifications []matrixSpecification,
	outputDirectory string,
	writeMatrix matrixFileWriter,
	renameFile fileRenameFunction,
) error {
	return writeMatrixSetWithPreparedPublications(
		specifications,
		outputDirectory,
		writeMatrix,
		renameFile,
		nil,
	)
}

func writeMatrixSetWithPreparedPublications(
	specifications []matrixSpecification,
	outputDirectory string,
	writeMatrix matrixFileWriter,
	renameFile fileRenameFunction,
	preparedPublications []matrixPublication,
) (resultError error) {
	if len(specifications) == 0 {
		return fmt.Errorf("no matrices were provided")
	}
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	publications := make([]matrixPublication, len(preparedPublications)+len(specifications))
	copy(publications, preparedPublications)
	defer func() {
		resultError = errors.Join(resultError, cleanupTemporaryArtifacts(publications))
	}()

	for matrixIndex, specification := range specifications {
		if specification.dataFrame == nil {
			return fmt.Errorf("matrix %q is nil", specification.filename)
		}
		if specification.filename == "" || filepath.Base(specification.filename) != specification.filename {
			return fmt.Errorf("invalid matrix filename %q", specification.filename)
		}

		targetPath := filepath.Join(outputDirectory, specification.filename)
		if err := validateFinalTarget(targetPath); err != nil {
			return err
		}

		temporaryPath, err := writeTemporaryMatrix(
			specification.dataFrame,
			outputDirectory,
			specification.filename,
			writeMatrix,
		)
		if err != nil {
			return err
		}
		publicationIndex := len(preparedPublications) + matrixIndex
		publications[publicationIndex] = matrixPublication{
			targetPath:    targetPath,
			temporaryPath: temporaryPath,
		}
	}

	for publicationIndex := range publications {
		if err := validateFinalTarget(publications[publicationIndex].targetPath); err != nil {
			return err
		}
	}

	if err := backupExistingTargets(publications, outputDirectory, renameFile); err != nil {
		rollbackError := rollbackPublications(publications, outputDirectory, renameFile)
		return errors.Join(err, rollbackError)
	}

	for publicationIndex := range publications {
		publication := &publications[publicationIndex]
		if err := renameFile(publication.temporaryPath, publication.targetPath); err != nil {
			publishError := fmt.Errorf("failed to publish %s: %w", filepath.Base(publication.targetPath), err)
			rollbackError := rollbackPublications(publications, outputDirectory, renameFile)
			return errors.Join(publishError, rollbackError)
		}
		publication.temporaryPath = ""
		publication.published = true
	}

	if err := syncDirectory(outputDirectory); err != nil {
		rollbackError := rollbackPublications(publications, outputDirectory, renameFile)
		return errors.Join(fmt.Errorf("failed to sync published matrices: %w", err), rollbackError)
	}

	for publicationIndex := range publications {
		publication := &publications[publicationIndex]
		if publication.backupPath == "" {
			continue
		}
		if err := os.Remove(publication.backupPath); err != nil {
			return fmt.Errorf("failed to remove backup for %s: %w", filepath.Base(publication.targetPath), err)
		}
		publication.backupPath = ""
	}

	if err := syncDirectory(outputDirectory); err != nil {
		return fmt.Errorf("failed to sync output directory after cleanup: %w", err)
	}
	return nil
}

func writeTemporaryMatrix(
	dataFrame *dataframe.DataFrame,
	outputDirectory string,
	filename string,
	writeMatrix matrixFileWriter,
) (string, error) {
	temporaryFile, err := os.CreateTemp(outputDirectory, "."+filename+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file for %s: %w", filename, err)
	}
	temporaryPath := temporaryFile.Name()
	if err := temporaryFile.Close(); err != nil {
		removeError := removeIfPresent(temporaryPath)
		return "", errors.Join(
			fmt.Errorf("failed to close temporary file for %s: %w", filename, err),
			removeError,
		)
	}

	if err := writeMatrix(dataFrame, temporaryPath); err != nil {
		removeError := removeIfPresent(temporaryPath)
		return "", errors.Join(
			fmt.Errorf("failed to write temporary matrix %s: %w", filename, err),
			removeError,
		)
	}
	return temporaryPath, nil
}

func validateFinalTarget(targetPath string) error {
	fileInformation, err := os.Lstat(targetPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to inspect output target %s: %w", targetPath, err)
	}
	if fileInformation.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symbolic link output target %s", targetPath)
	}
	if !fileInformation.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular output target %s", targetPath)
	}
	return nil
}

func backupExistingTargets(
	publications []matrixPublication,
	outputDirectory string,
	renameFile fileRenameFunction,
) error {
	for publicationIndex := range publications {
		publication := &publications[publicationIndex]
		fileInformation, err := os.Lstat(publication.targetPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to inspect output target %s before publication: %w", publication.targetPath, err)
		}
		if fileInformation.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symbolic link output target %s", publication.targetPath)
		}
		if !fileInformation.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular output target %s", publication.targetPath)
		}

		backupPath, err := reserveBackupPath(outputDirectory, filepath.Base(publication.targetPath))
		if err != nil {
			return err
		}
		if err := renameFile(publication.targetPath, backupPath); err != nil {
			removeError := removeIfPresent(backupPath)
			return errors.Join(
				fmt.Errorf("failed to back up %s: %w", filepath.Base(publication.targetPath), err),
				removeError,
			)
		}
		publication.backupPath = backupPath
		publication.hadOriginal = true
	}
	return nil
}

func reserveBackupPath(outputDirectory, filename string) (string, error) {
	backupFile, err := os.CreateTemp(outputDirectory, "."+filename+".*.backup")
	if err != nil {
		return "", fmt.Errorf("failed to reserve backup path for %s: %w", filename, err)
	}
	backupPath := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		removeError := removeIfPresent(backupPath)
		return "", errors.Join(
			fmt.Errorf("failed to close backup placeholder for %s: %w", filename, err),
			removeError,
		)
	}
	if err := os.Remove(backupPath); err != nil {
		return "", fmt.Errorf("failed to release backup placeholder for %s: %w", filename, err)
	}
	return backupPath, nil
}

func rollbackPublications(
	publications []matrixPublication,
	outputDirectory string,
	renameFile fileRenameFunction,
) error {
	var rollbackError error

	for publicationIndex := len(publications) - 1; publicationIndex >= 0; publicationIndex-- {
		publication := &publications[publicationIndex]
		if !publication.published {
			continue
		}
		if err := removeIfPresent(publication.targetPath); err != nil {
			rollbackError = errors.Join(
				rollbackError,
				fmt.Errorf("failed to remove newly published %s: %w", filepath.Base(publication.targetPath), err),
			)
			continue
		}
		publication.published = false
	}

	for publicationIndex := len(publications) - 1; publicationIndex >= 0; publicationIndex-- {
		publication := &publications[publicationIndex]
		if !publication.hadOriginal || publication.backupPath == "" {
			continue
		}
		if err := renameFile(publication.backupPath, publication.targetPath); err != nil {
			rollbackError = errors.Join(
				rollbackError,
				fmt.Errorf("failed to restore original %s: %w", filepath.Base(publication.targetPath), err),
			)
			continue
		}
		publication.backupPath = ""
		publication.hadOriginal = false
	}

	if err := syncDirectory(outputDirectory); err != nil {
		rollbackError = errors.Join(rollbackError, fmt.Errorf("failed to sync rollback: %w", err))
	}
	return rollbackError
}

func cleanupTemporaryArtifacts(publications []matrixPublication) error {
	var cleanupError error
	for publicationIndex := range publications {
		temporaryPath := publications[publicationIndex].temporaryPath
		if temporaryPath == "" {
			continue
		}
		if err := removeIfPresent(temporaryPath); err != nil {
			cleanupError = errors.Join(cleanupError, err)
		}
	}
	return cleanupError
}

func removeIfPresent(filePath string) error {
	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %s: %w", filePath, err)
	}
	return nil
}

func syncDirectory(directoryPath string) (resultError error) {
	directory, err := os.Open(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to open output directory: %w", err)
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		if closeError := directory.Close(); closeError != nil {
			resultError = errors.Join(resultError, fmt.Errorf("failed to close output directory: %w", closeError))
		}
	}()

	if err := directory.Sync(); err != nil {
		resultError = errors.Join(resultError, fmt.Errorf("failed to sync output directory: %w", err))
	}
	if err := directory.Close(); err != nil {
		resultError = errors.Join(resultError, fmt.Errorf("failed to close output directory: %w", err))
	}
	closed = true
	return resultError
}

// WriteMatricesWithManifest atomically publishes both matrices and their provenance manifest.
func WriteMatricesWithManifest(countDataFrame, normalizedDataFrame *dataframe.DataFrame, outputDirectory string, manifest any) error {
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	manifestTargetPath := filepath.Join(outputDirectory, "matrix_manifest.json")
	if err := validateFinalTarget(manifestTargetPath); err != nil {
		return err
	}
	encodedManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode matrix manifest: %w", err)
	}
	manifestTemporaryPath, err := writeTemporaryBytes(
		outputDirectory,
		"matrix_manifest.json",
		encodedManifest,
	)
	if err != nil {
		return err
	}
	return writeMatrixSetWithPreparedPublications(
		[]matrixSpecification{
			{dataFrame: countDataFrame, filename: countMatrixFilename},
			{dataFrame: normalizedDataFrame, filename: normalizedMatrixFilename},
		},
		outputDirectory,
		writeDataFrame,
		os.Rename,
		[]matrixPublication{{
			targetPath:    manifestTargetPath,
			temporaryPath: manifestTemporaryPath,
		}},
	)
}

func writeTemporaryBytes(outputDirectory, filename string, contents []byte) (string, error) {
	temporaryFile, err := os.CreateTemp(outputDirectory, "."+filename+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file for %s: %w", filename, err)
	}
	temporaryPath := temporaryFile.Name()
	cleanup := func() error { return removeIfPresent(temporaryPath) }
	if _, err := temporaryFile.Write(contents); err != nil {
		closeError := temporaryFile.Close()
		return "", errors.Join(
			fmt.Errorf("failed to write temporary file %s: %w", filename, err),
			closeError,
			cleanup(),
		)
	}
	if err := temporaryFile.Sync(); err != nil {
		closeError := temporaryFile.Close()
		return "", errors.Join(
			fmt.Errorf("failed to sync temporary file %s: %w", filename, err),
			closeError,
			cleanup(),
		)
	}
	if err := temporaryFile.Close(); err != nil {
		return "", errors.Join(
			fmt.Errorf("failed to close temporary file %s: %w", filename, err),
			cleanup(),
		)
	}
	return temporaryPath, nil
}

// WriteCountMatrix writes the raw count matrix.
func WriteCountMatrix(dataFrame *dataframe.DataFrame, outputDirectory string) error {
	return WriteMatrix(dataFrame, outputDirectory, countMatrixFilename)
}

// WriteNormalizedMatrix writes the log2-normalized matrix.
func WriteNormalizedMatrix(dataFrame *dataframe.DataFrame, outputDirectory string) error {
	return WriteMatrix(dataFrame, outputDirectory, normalizedMatrixFilename)
}
